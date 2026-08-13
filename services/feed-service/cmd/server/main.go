// Command server runs Feed Service: a gRPC server that reads the fanout-on-write timeline
// cache in Redis, merges in fanout-on-read candidates from celebrity accounts, hydrates
// content, ranks, and paginates the result. See IMPLEMENTATION_PLAN.md §5.4.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	feedv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/feed/v1"
	postv1 "github.com/AnishK05/cascade-feed-ranking-engine/proto/gen/go/post/v1"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/candidate"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/config"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feedserver"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/hydrator"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/observability"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/ranking"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/signals"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg, err := config.Load()
	if err != nil {
		logger.Error("load configuration", "error", err)
		os.Exit(1)
	}
	startupCtx, cancelStartup := context.WithTimeout(context.Background(), cfg.StartupTimeout)
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

	postConn, err := grpc.DialContext(
		startupCtx, cfg.PostServiceAddr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(),
	)
	if err != nil {
		logger.Error("connect to Post Service", "address", cfg.PostServiceAddr, "error", err)
		os.Exit(1)
	}
	defer postConn.Close()

	lis, err := net.Listen("tcp", cfg.Addr())
	if err != nil {
		logger.Error("failed to listen", "address", cfg.Addr(), "error", err)
		os.Exit(1)
	}
	defer lis.Close()

	metrics := observability.NewMetrics()
	metricsServer := observability.ServeMetrics(cfg.MetricsAddr, logger)
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = metricsServer.Shutdown(shutdownCtx)
	}()

	var candidates feedserver.CandidateStore = candidate.NewRedis(redisClient)
	var hydrate feedserver.Hydrator = hydrator.NewRedisPost(
		redisClient, postv1.NewPostServiceClient(postConn), cfg.CacheTTL,
	)
	if cfg.BypassCache {
		logger.Warn("FEED_BYPASS_CACHE enabled; GetFeed will read candidates and post bodies from PostgreSQL")
		pgCandidates := candidate.NewPostgres(pool)
		pgCandidates.OnQuery = observability.RecordPostgresQuery
		pgHydrate := hydrator.NewPostgres(pool)
		pgHydrate.OnQuery = observability.RecordPostgresQuery
		candidates = pgCandidates
		hydrate = pgHydrate
	}
	signalStore := signals.NewPostgres(pool, cfg.AffinityWindow, cfg.AffinityDefault)
	signalStore.OnQuery = observability.RecordPostgresQuery

	grpcServer := grpc.NewServer(grpc.UnaryInterceptor(observability.UnaryServerInterceptor(logger)))
	feedv1.RegisterFeedServiceServer(grpcServer, feedserver.New(
		candidates,
		hydrate,
		signalStore,
		ranking.NewHeuristic(ranking.Weights{
			Recency: cfg.RecencyWeight, Engagement: cfg.EngagementWeight,
			Affinity: cfg.AffinityWeight, HalfLife: cfg.RecencyHalfLife,
		}),
		metrics, cfg.CandidatePool, cfg.DefaultPageSize, cfg.MaxPageSize, logger,
	))

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("feed-service listening", "address", cfg.Addr())
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
		logger.Info("shutting down feed-service")
		stopped := make(chan struct{})
		go func() {
			grpcServer.GracefulStop()
			close(stopped)
		}()
		select {
		case <-stopped:
		case <-time.After(cfg.StartupTimeout):
			logger.Warn("graceful shutdown timed out; forcing stop")
			grpcServer.Stop()
		}
	}
}
