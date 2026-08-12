// Command worker runs the Fanout Worker: a Kafka consumer group that fans posts out to
// follower timeline caches in Redis (fanout-on-write), with a hybrid fallback to
// fanout-on-read for celebrity accounts. See IMPLEMENTATION_PLAN.md §5.3.
package main

import (
	"context"
	"errors"
	"log"
	"os/signal"
	"syscall"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/config"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/consumer"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/fanout"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/repository"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/timeline"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	log.Printf(
		"fanout-worker: starting (kafka_brokers=%v celebrity_follower_threshold=%d max_timeline_len=%d)",
		cfg.KafkaBrokers, cfg.CelebrityFollowerThreshold, cfg.MaxTimelineLen,
	)

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("fanout-worker: configure Postgres: %v", err)
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
		log.Fatalf("fanout-worker: configure Kafka: %v", err)
	}
	defer kafkaClient.Close()

	startupCtx, cancel := context.WithTimeout(ctx, cfg.StartupTimeout)
	defer cancel()
	if err := pool.Ping(startupCtx); err != nil {
		log.Fatalf("fanout-worker: ping Postgres: %v", err)
	}
	if err := redisClient.Ping(startupCtx).Err(); err != nil {
		log.Fatalf("fanout-worker: ping Redis: %v", err)
	}
	if err := kafkaClient.Ping(startupCtx); err != nil {
		log.Fatalf("fanout-worker: ping Kafka: %v", err)
	}

	repo := repository.NewPostgres(pool)
	timelines := timeline.NewRedis(redisClient, cfg.FollowerCountCacheTTL)
	processor := fanout.NewProcessor(repo, timelines, fanout.Settings{
		PostTopic: cfg.PostTopic, FollowTopic: cfg.FollowTopic,
		CelebrityFollowerThreshold: cfg.CelebrityFollowerThreshold,
		MaxTimelineLen:             cfg.MaxTimelineLen, BackfillCount: cfg.BackfillCount,
		FanoutBatchSize: cfg.FanoutBatchSize,
	})
	publisher := consumer.NewKafkaPublisher(
		kafkaClient, cfg.PostTopic, cfg.FollowTopic, cfg.PostDLQTopic, cfg.FollowDLQTopic,
	)
	handler := consumer.NewRecordHandler(processor, publisher, cfg.MaxRetries, cfg.RetryBackoff)

	log.Printf("fanout-worker: consuming topics %q and %q as group %q",
		cfg.PostTopic, cfg.FollowTopic, cfg.ConsumerGroup)
	source := consumer.NewKafkaRecordSource(kafkaClient)
	err = consumer.NewRunner(source, handler).Run(ctx)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatalf("fanout-worker: stopped with error: %v", err)
	}
	log.Println("fanout-worker: shutdown complete")
}
