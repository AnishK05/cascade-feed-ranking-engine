// Package config loads Feed Service configuration from environment variables, falling back to
// sane defaults for local development (matching the values used in deploy/docker-compose.yml).
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
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
	DefaultPageSize  int32
	MaxPageSize      int32
	CandidatePool    int
	CacheTTL         time.Duration
	RecencyWeight    float64
	EngagementWeight float64
	AffinityWeight   float64
	RecencyHalfLife  time.Duration
	AffinityWindow   time.Duration
	AffinityDefault  float64
	StartupTimeout   time.Duration
	MetricsAddr      string
	// BypassCache skips Redis timelines and the post cache so GetFeed reads
	// candidates and post bodies from PostgreSQL. Used for the Phase 12 baseline
	// benchmark (IMPLEMENTATION_PLAN.md §13.3).
	BypassCache bool
}

// Load reads configuration from the environment, applying defaults for any unset variable.
func Load() (Config, error) {
	cfg := Config{
		GRPCPort:        getEnv("FEED_SERVICE_GRPC_PORT", "9091"),
		DatabaseURL:     getEnv("DATABASE_URL", "postgres://cascade:cascade@localhost:5432/cascade?sslmode=disable"),
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		PostServiceAddr: getEnv("POST_SERVICE_ADDR", "localhost:9090"),
		MetricsAddr:     getEnv("FEED_METRICS_ADDR", ":9101"),
		BypassCache:     boolEnv("FEED_BYPASS_CACHE", false),
	}
	var err error
	if cfg.DefaultPageSize, err = int32Env("FEED_DEFAULT_PAGE_SIZE", 20); err != nil {
		return Config{}, err
	}
	if cfg.MaxPageSize, err = int32Env("FEED_MAX_PAGE_SIZE", 100); err != nil {
		return Config{}, err
	}
	if cfg.CandidatePool, err = intEnv("FEED_CANDIDATE_POOL_SIZE", 200); err != nil {
		return Config{}, err
	}
	if cfg.CacheTTL, err = durationEnv("FEED_POST_CACHE_TTL", 6*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.RecencyWeight, err = floatEnv("FEED_RECENCY_WEIGHT", 1); err != nil {
		return Config{}, err
	}
	if cfg.EngagementWeight, err = floatEnv("FEED_ENGAGEMENT_WEIGHT", 1); err != nil {
		return Config{}, err
	}
	if cfg.AffinityWeight, err = floatEnv("FEED_AFFINITY_WEIGHT", 1); err != nil {
		return Config{}, err
	}
	if cfg.RecencyHalfLife, err = durationEnv("FEED_RECENCY_HALF_LIFE", 12*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.AffinityWindow, err = durationEnv("FEED_AFFINITY_WINDOW", 30*24*time.Hour); err != nil {
		return Config{}, err
	}
	if cfg.AffinityDefault, err = floatEnv("FEED_AFFINITY_DEFAULT", 0); err != nil {
		return Config{}, err
	}
	if cfg.StartupTimeout, err = durationEnv("FEED_STARTUP_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.DefaultPageSize <= 0 || cfg.MaxPageSize < cfg.DefaultPageSize ||
		cfg.CandidatePool <= 0 || cfg.CacheTTL <= 0 || cfg.RecencyHalfLife <= 0 ||
		cfg.AffinityWindow <= 0 || cfg.StartupTimeout <= 0 {
		return Config{}, fmt.Errorf("feed configuration values must be positive and max page size must not be below default")
	}
	return cfg, nil
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

func int32Env(key string, fallback int32) (int32, error) {
	value, err := strconv.ParseInt(getEnv(key, strconv.FormatInt(int64(fallback), 10)), 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return int32(value), nil
}

func intEnv(key string, fallback int) (int, error) {
	value, err := strconv.Atoi(getEnv(key, strconv.Itoa(fallback)))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func floatEnv(key string, fallback float64) (float64, error) {
	value, err := strconv.ParseFloat(getEnv(key, strconv.FormatFloat(fallback, 'g', -1, 64)), 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value, err := time.ParseDuration(getEnv(key, fallback.String()))
	if err != nil {
		return 0, fmt.Errorf("%s: %w", key, err)
	}
	return value, nil
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
