// Package config loads Post Service configuration from environment variables, falling back to
// sane defaults for local development (matching the values used in deploy/docker-compose.yml).
package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// Config holds all runtime configuration for Post Service.
type Config struct {
	// GRPCPort is the TCP port the gRPC server listens on.
	GRPCPort string
	// DatabaseURL is a Postgres connection string, e.g.
	// "postgres://user:pass@localhost:5432/cascade?sslmode=disable".
	DatabaseURL string
	// RedisAddr is the host:port of the Redis instance used for the write-through post cache.
	RedisAddr string
	// KafkaBrokers is a comma-separated list of Kafka broker addresses used to publish
	// PostCreated/PostDeleted events.
	KafkaBrokers string
	// KafkaTopic receives JSON PostCreated and PostDeleted events.
	KafkaTopic string
	// CacheTTL controls the lifetime of post JSON values in Redis.
	CacheTTL time.Duration
	// TombstoneTTL is how long deleted post IDs stay in the global Redis tombstones
	// set. It should outlast typical timeline turnover (IMPLEMENTATION_PLAN.md §7.4).
	TombstoneTTL time.Duration
	// MetricsAddr is the bind address for the Prometheus /metrics HTTP server.
	MetricsAddr string
	// BypassCache skips the Redis post cache so GetPosts always queries PostgreSQL.
	// Used for the Phase 12 baseline benchmark (IMPLEMENTATION_PLAN.md §13.3).
	BypassCache bool
}

// Load reads configuration from the environment, applying defaults for any unset variable.
func Load() Config {
	cacheTTL, err := time.ParseDuration(getEnv("POST_CACHE_TTL", "6h"))
	if err != nil || cacheTTL <= 0 {
		cacheTTL = 6 * time.Hour
	}
	tombstoneTTL, err := time.ParseDuration(getEnv("POST_TOMBSTONE_TTL", "24h"))
	if err != nil || tombstoneTTL <= 0 {
		tombstoneTTL = 24 * time.Hour
	}
	return Config{
		GRPCPort:     getEnv("POST_SERVICE_GRPC_PORT", "9090"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable"),
		RedisAddr:    getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
		KafkaTopic:   getEnv("KAFKA_TOPIC", "post-events"),
		CacheTTL:     cacheTTL,
		TombstoneTTL: tombstoneTTL,
		MetricsAddr:  getEnv("POST_METRICS_ADDR", ":9100"),
		BypassCache:  boolEnv("POST_BYPASS_CACHE", false),
	}
}

// Addr returns the address the gRPC server should bind to, e.g. ":9090".
func (c Config) Addr() string {
	return fmt.Sprintf(":%s", c.GRPCPort)
}

// Brokers parses the comma-separated broker setting and discards empty entries.
func (c Config) Brokers() []string {
	raw := strings.Split(c.KafkaBrokers, ",")
	brokers := make([]string, 0, len(raw))
	for _, broker := range raw {
		if broker = strings.TrimSpace(broker); broker != "" {
			brokers = append(brokers, broker)
		}
	}
	return brokers
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func boolEnv(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(getEnv(key, "")))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
