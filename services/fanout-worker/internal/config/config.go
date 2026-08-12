// Package config loads Fanout Worker configuration from environment variables, falling back
// to sane defaults for local development (matching the values used in deploy/docker-compose.yml).
package config

import (
	"os"
	"strconv"
)

// Config holds all runtime configuration for the Fanout Worker.
type Config struct {
	// KafkaBrokers is a comma-separated list of Kafka broker addresses to consume from.
	KafkaBrokers string
	// DatabaseURL is a Postgres connection string. Phase 5 uses this to read follower lists
	// directly (a deliberate, temporary coupling refactored away in Phase 9.5 — see
	// IMPLEMENTATION_PLAN.md §7.2 and the Decisions Log, §19).
	DatabaseURL string
	// RedisAddr is the host:port of the Redis instance holding per-user timelines and the
	// celebrity-posts fanout-on-read source.
	RedisAddr string
	// CelebrityFollowerThreshold is the follower count at or above which an author's posts
	// are fanned out via fanout-on-read instead of fanout-on-write (see §5.3 / §8 glossary).
	CelebrityFollowerThreshold int64
	// MaxTimelineLen bounds how many entries are kept per follower's Redis ZSET.
	MaxTimelineLen int64
	// BackfillCount is how many of a followee's recent posts get backfilled into a new
	// follower's timeline on FollowCreated (cache warming, §7.3).
	BackfillCount int64
}

// Load reads configuration from the environment, applying defaults for any unset variable.
func Load() Config {
	return Config{
		KafkaBrokers:               getEnv("KAFKA_BROKERS", "localhost:9092"),
		DatabaseURL:                getEnv("DATABASE_URL", "postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable"),
		RedisAddr:                  getEnv("REDIS_ADDR", "localhost:6379"),
		CelebrityFollowerThreshold: getEnvInt64("CELEBRITY_FOLLOWER_THRESHOLD", 10_000),
		MaxTimelineLen:             getEnvInt64("MAX_TIMELINE_LEN", 1_000),
		BackfillCount:              getEnvInt64("BACKFILL_COUNT", 20),
	}
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt64(key string, fallback int64) int64 {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}
