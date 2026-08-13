// Command warm-cache rebuilds Redis timeline caches from Postgres after a cold start
// or benchmark reset. See IMPLEMENTATION_PLAN.md §7.3.
package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/config"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/repository"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/timeline"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/warm"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	reset := flag.Bool("reset", true, "delete existing timeline/celebrity Redis keys before rewriting them")
	warmPosts := flag.Bool("warm-posts", true, "also write post:{id} content cache entries")
	postTTL := flag.Duration("post-ttl", 6*time.Hour, "TTL for warmed post:{id} keys")
	flag.Parse()

	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("warm-cache: configure Postgres: %v", err)
	}
	defer pool.Close()
	redisClient := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr, Password: cfg.RedisPassword, DB: cfg.RedisDB,
	})
	defer redisClient.Close()

	startupCtx, cancel := context.WithTimeout(ctx, cfg.StartupTimeout)
	defer cancel()
	if err := pool.Ping(startupCtx); err != nil {
		log.Fatalf("warm-cache: ping Postgres: %v", err)
	}
	if err := redisClient.Ping(startupCtx).Err(); err != nil {
		log.Fatalf("warm-cache: ping Redis: %v", err)
	}

	warmer := warm.New(
		repository.NewPostgres(pool),
		timeline.NewRedis(redisClient, cfg.FollowerCountCacheTTL, cfg.TombstoneTTL),
		redisClient,
		warm.Settings{
			CelebrityFollowerThreshold: cfg.CelebrityFollowerThreshold,
			MaxTimelineLen:             cfg.MaxTimelineLen,
			BackfillCount:              cfg.BackfillCount,
			BatchSize:                  cfg.FanoutBatchSize,
			PostCacheTTL:               *postTTL,
			Reset:                      *reset,
			WarmPosts:                  *warmPosts,
		},
	)
	result, err := warmer.Run(ctx)
	if err != nil {
		log.Fatalf("warm-cache: %v", err)
	}
	log.Printf(
		"warm-cache: rebuilt %d user timelines, %d celebrity posts, %d post cache entries (users=%d reset=%t)",
		result.Timelines, result.CelebrityPosts, result.PostCache, result.Users, *reset,
	)
	os.Exit(0)
}
