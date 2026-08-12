// Command server runs Post Service: a gRPC server that owns post content, writes it to
// Postgres, write-through caches it in Redis, and publishes PostCreated/PostDeleted events to
// Kafka for the Fanout Worker to consume. See IMPLEMENTATION_PLAN.md §5.1.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	postv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/post/v1"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/cache"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/config"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/events"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/postserver"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/post-service/internal/repository"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()

	startupCtx, cancelStartup := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelStartup()

	pool, err := pgxpool.New(startupCtx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("create PostgreSQL pool", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	if err := pool.Ping(startupCtx); err != nil {
		logger.Error("PostgreSQL startup ping failed", "error", err)
		os.Exit(1)
	}

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	defer redisClient.Close()
	if err := redisClient.Ping(startupCtx).Err(); err != nil {
		logger.Error("Redis startup ping failed", "error", err)
		os.Exit(1)
	}

	kafkaClient, err := kgo.NewClient(kgo.SeedBrokers(cfg.Brokers()...))
	if err != nil {
		logger.Error("create Kafka producer", "error", err)
		os.Exit(1)
	}
	defer kafkaClient.Close()
	if err := kafkaClient.Ping(startupCtx); err != nil {
		logger.Error("Kafka startup ping failed", "error", err)
		os.Exit(1)
	}

	lis, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		logger.Error("failed to listen", "address", cfg.Addr(), "error", err)
		os.Exit(1)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer()
	postv1.RegisterPostServiceServer(grpcServer, postserver.New(
		repository.New(pool),
		cache.New(redisClient, cfg.CacheTTL),
		events.NewKafka(kafkaClient, cfg.KafkaTopic),
		logger,
	))

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("post-service listening", "address", cfg.Addr())
		serveErr <- grpcServer.Serve(lis)
	}()

	signalCtx, stopSignals := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stopSignals()
	select {
	case err := <-serveErr:
		if err != nil {
			logger.Error("gRPC server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	case <-signalCtx.Done():
		logger.Info("shutting down post-service")
		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(10 * time.Second):
			logger.Warn("graceful shutdown timed out; forcing stop")
			grpcServer.Stop()
		}
	}
}
