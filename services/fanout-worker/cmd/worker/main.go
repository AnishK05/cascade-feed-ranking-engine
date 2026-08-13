// Command worker runs the Fanout Worker: a Kafka consumer group that fans posts out to
// follower timeline caches in Redis (fanout-on-write), with a hybrid fallback to
// fanout-on-read for celebrity accounts. See IMPLEMENTATION_PLAN.md §5.3.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/config"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/consumer"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/fanout"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/observability"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/repository"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/timeline"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logger.Info("starting",
		"kafka_brokers", cfg.KafkaBrokers,
		"celebrity_follower_threshold", cfg.CelebrityFollowerThreshold,
		"max_timeline_len", cfg.MaxTimelineLen,
	)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("configure Postgres", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB,
	})
	defer redisClient.Close()

	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers(cfg.KafkaBrokers...),
		kgo.ConsumerGroup(cfg.ConsumerGroup),
		kgo.ConsumeTopics(cfg.PostTopic, cfg.FollowTopic),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
	)
	if err != nil {
		logger.Error("configure Kafka", "error", err)
		os.Exit(1)
	}
	defer kafkaClient.Close()

	startupCtx, cancel := context.WithTimeout(ctx, cfg.StartupTimeout)
	defer cancel()
	if err := pool.Ping(startupCtx); err != nil {
		logger.Error("ping Postgres", "error", err)
		os.Exit(1)
	}
	if err := redisClient.Ping(startupCtx).Err(); err != nil {
		logger.Error("ping Redis", "error", err)
		os.Exit(1)
	}
	if err := kafkaClient.Ping(startupCtx); err != nil {
		logger.Error("ping Kafka", "error", err)
		os.Exit(1)
	}

	metrics := observability.NewMetrics()
	metricsServer := observability.ServeMetrics(cfg.MetricsAddr, logger)
	defer func() {
		shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancelShutdown()
		_ = metricsServer.Shutdown(shutdownCtx)
	}()
	go observability.WatchLag(ctx, kafkaClient, cfg.ConsumerGroup, metrics, logger)

	repo := repository.NewPostgres(pool)
	timelines := timeline.NewRedis(redisClient, cfg.FollowerCountCacheTTL, cfg.TombstoneTTL)
	processor := observability.Instrument(fanout.NewProcessor(repo, timelines, fanout.Settings{
		PostTopic: cfg.PostTopic, FollowTopic: cfg.FollowTopic,
		CelebrityFollowerThreshold: cfg.CelebrityFollowerThreshold,
		MaxTimelineLen:             cfg.MaxTimelineLen, BackfillCount: cfg.BackfillCount,
		FanoutBatchSize: cfg.FanoutBatchSize,
	}), metrics)
	publisher := consumer.NewKafkaPublisher(
		kafkaClient, cfg.PostTopic, cfg.FollowTopic, cfg.PostDLQTopic, cfg.FollowDLQTopic,
	)
	handler := consumer.NewRecordHandler(processor, publisher, cfg.MaxRetries, cfg.RetryBackoff)

	logger.Info("consuming", "post_topic", cfg.PostTopic, "follow_topic", cfg.FollowTopic, "group", cfg.ConsumerGroup)
	source := consumer.NewKafkaRecordSource(kafkaClient)
	err = consumer.NewRunner(source, handler).Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("stopped with error", "error", err)
		os.Exit(1)
	}
	logger.Info("shutdown complete")
}
