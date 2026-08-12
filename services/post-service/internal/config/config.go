// Package config loads Post Service configuration from environment variables, falling back to
// sane defaults for local development (matching the values used in deploy/docker-compose.yml).
package config

import (
	"fmt"
	"os"
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
}

// Load reads configuration from the environment, applying defaults for any unset variable.
func Load() Config {
	return Config{
		GRPCPort:     getEnv("POST_SERVICE_GRPC_PORT", "9090"),
		DatabaseURL:  getEnv("DATABASE_URL", "postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable"),
		RedisAddr:    getEnv("REDIS_ADDR", "localhost:6379"),
		KafkaBrokers: getEnv("KAFKA_BROKERS", "localhost:9092"),
	}
}

// Addr returns the address the gRPC server should bind to, e.g. ":9090".
func (c Config) Addr() string {
	return fmt.Sprintf(":%s", c.GRPCPort)
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}
