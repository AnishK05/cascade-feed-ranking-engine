// Package config loads Feed Service configuration from environment variables, falling back to
// sane defaults for local development (matching the values used in deploy/docker-compose.yml).
package config

import (
	"fmt"
	"os"
)

// Config holds all runtime configuration for Feed Service.
type Config struct {
	// GRPCPort is the TCP port the gRPC server listens on.
	GRPCPort string
	// DatabaseURL is a Postgres connection string, used as the cache-miss fallback when
	// hydrating post content (see IMPLEMENTATION_PLAN.md §5.4).
	DatabaseURL string
	// RedisAddr is the host:port of the Redis instance holding per-user timelines, the
	// celebrity-posts fanout-on-read source, and the post content cache.
	RedisAddr string
	// PostServiceAddr is the gRPC address of Post Service, used to batch-hydrate cache misses.
	PostServiceAddr string
	// DefaultPageSize and MaxPageSize bound GetFeed pagination.
	DefaultPageSize int32
	MaxPageSize     int32
}

// Load reads configuration from the environment, applying defaults for any unset variable.
func Load() Config {
	return Config{
		GRPCPort:        getEnv("FEED_SERVICE_GRPC_PORT", "9091"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		PostServiceAddr: getEnv("POST_SERVICE_ADDR", "localhost:9090"),
		DefaultPageSize: 20,
		MaxPageSize:     100,
	}
}

// Addr returns the address the gRPC server should bind to, e.g. ":9091".
func (c Config) Addr() string {
	return fmt.Sprintf(":%s", c.GRPCPort)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
