// Package config loads Fanout Worker configuration from environment variables, falling back
// to sane defaults for local development (matching the values used in deploy/docker-compose.yml).
package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all runtime configuration for the Fanout Worker.
type Config struct {
	KafkaBrokers   []string
	PostTopic      string
	FollowTopic    string
	PostDLQTopic   string
	FollowDLQTopic string
	ConsumerGroup  string
	// DatabaseURL is a Postgres connection string. Phase 5 uses this to read follower lists
	// directly (a deliberate, temporary coupling refactored away in Phase 9.5 — see
	// IMPLEMENTATION_PLAN.md §7.2 and the Decisions Log, §19).
	DatabaseURL string
	// RedisAddr is the host:port of the Redis instance holding per-user timelines and the
	// celebrity-posts fanout-on-read source.
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	// CelebrityFollowerThreshold is the follower count at or above which an author's posts
	// are fanned out via fanout-on-read instead of fanout-on-write (see §5.3 / §8 glossary).
	CelebrityFollowerThreshold int64
	// MaxTimelineLen bounds how many entries are kept per follower's Redis ZSET.
	MaxTimelineLen int64
	// BackfillCount is how many of a followee's recent posts get backfilled into a new
	// follower's timeline on FollowCreated (cache warming, §7.3).
	BackfillCount   int64
	FanoutBatchSize int
	MaxRetries      int
	RetryBackoff    time.Duration
	// FollowerCountCacheTTL is the lifetime of the short-lived celebrity-check cache.
	FollowerCountCacheTTL time.Duration
	// TombstoneTTL is how long deleted post IDs stay in the global Redis tombstones
	// set used by Feed Service at read time (IMPLEMENTATION_PLAN.md §7.4).
	TombstoneTTL   time.Duration
	StartupTimeout time.Duration
}

// Load reads configuration from the environment, applying defaults for any unset variable.
func Load() Config {
	return Config{
		KafkaBrokers:               splitCSV(getEnv("KAFKA_BROKERS", "localhost:9092")),
		PostTopic:                  getEnv("POST_EVENTS_TOPIC", "post-events"),
		FollowTopic:                getEnv("FOLLOW_EVENTS_TOPIC", "follow-events"),
		PostDLQTopic:               getEnv("POST_EVENTS_DLQ_TOPIC", "post-events.dlq"),
		FollowDLQTopic:             getEnv("FOLLOW_EVENTS_DLQ_TOPIC", "follow-events.dlq"),
		ConsumerGroup:              getEnv("KAFKA_CONSUMER_GROUP", "fanout-worker"),
		DatabaseURL:                getEnv("DATABASE_URL", "postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable"),
		RedisAddr:                  getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:              getEnv("REDIS_PASSWORD", ""),
		RedisDB:                    getEnvInt("REDIS_DB", 0),
		CelebrityFollowerThreshold: getEnvInt64("CELEBRITY_FOLLOWER_THRESHOLD", 10_000),
		MaxTimelineLen:             getEnvInt64("MAX_TIMELINE_LEN", 1_000),
		BackfillCount:              getEnvInt64("BACKFILL_COUNT", 20),
		FanoutBatchSize:            getEnvInt("FANOUT_BATCH_SIZE", 500),
		MaxRetries:                 getEnvNonNegativeInt("MAX_RETRIES", 3),
		RetryBackoff:               getEnvDuration("RETRY_BACKOFF", 250*time.Millisecond),
		FollowerCountCacheTTL:      getEnvDuration("FOLLOWER_COUNT_CACHE_TTL", time.Minute),
		TombstoneTTL:               getEnvDuration("TOMBSTONE_TTL", 24*time.Hour),
		StartupTimeout:             getEnvDuration("STARTUP_TIMEOUT", 5*time.Second),
	}
}

func splitCSV(value string) []string {
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" {
			result = append(result, part)
		}
	}
	return result
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

func getEnvInt(key string, fallback int) int {
	n := getEnvInt64(key, int64(fallback))
	if n <= 0 {
		return fallback
	}
	return int(n)
}

func getEnvNonNegativeInt(key string, fallback int) int {
	n := getEnvInt64(key, int64(fallback))
	if n < 0 {
		return fallback
	}
	return int(n)
}

func getEnvDuration(key string, fallback time.Duration) time.Duration {
	v := getEnv(key, "")
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return fallback
	}
	return d
}
