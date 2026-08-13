package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("POST_SERVICE_GRPC_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("KAFKA_BROKERS", "")
	t.Setenv("KAFKA_TOPIC", "")
	t.Setenv("POST_CACHE_TTL", "")
	t.Setenv("POST_TOMBSTONE_TTL", "")

	cfg := Load()

	if cfg.GRPCPort != "9090" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "9090")
	}
	if cfg.Addr() != ":9090" {
		t.Errorf("Addr() = %q, want %q", cfg.Addr(), ":9090")
	}
	if cfg.DatabaseURL == "" {
		t.Error("DatabaseURL should have a non-empty default")
	}
	if cfg.RedisAddr == "" {
		t.Error("RedisAddr should have a non-empty default")
	}
	if cfg.KafkaBrokers == "" {
		t.Error("KafkaBrokers should have a non-empty default")
	}
	if cfg.KafkaTopic != "post-events" {
		t.Errorf("KafkaTopic = %q, want post-events", cfg.KafkaTopic)
	}
	if cfg.CacheTTL != 6*time.Hour {
		t.Errorf("CacheTTL = %s, want 6h", cfg.CacheTTL)
	}
	if cfg.TombstoneTTL != 24*time.Hour {
		t.Errorf("TombstoneTTL = %s, want 24h", cfg.TombstoneTTL)
	}
	if cfg.MetricsAddr != ":9100" {
		t.Errorf("MetricsAddr = %q, want :9100", cfg.MetricsAddr)
	}
	if cfg.BypassCache {
		t.Error("BypassCache default should be false")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("POST_SERVICE_GRPC_PORT", "9999")
	t.Setenv("KAFKA_BROKERS", " kafka-1:9092, kafka-2:9092 ,,")
	t.Setenv("KAFKA_TOPIC", "custom-posts")
	t.Setenv("POST_CACHE_TTL", "45m")
	t.Setenv("POST_TOMBSTONE_TTL", "12h")
	t.Setenv("POST_BYPASS_CACHE", "true")

	cfg := Load()

	if cfg.GRPCPort != "9999" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "9999")
	}
	if cfg.Addr() != ":9999" {
		t.Errorf("Addr() = %q, want %q", cfg.Addr(), ":9999")
	}
	if got := cfg.Brokers(); len(got) != 2 || got[0] != "kafka-1:9092" || got[1] != "kafka-2:9092" {
		t.Errorf("Brokers() = %v", got)
	}
	if cfg.KafkaTopic != "custom-posts" || cfg.CacheTTL != 45*time.Minute || cfg.TombstoneTTL != 12*time.Hour {
		t.Errorf("topic/TTL = %q/%s/%s", cfg.KafkaTopic, cfg.CacheTTL, cfg.TombstoneTTL)
	}
	if !cfg.BypassCache {
		t.Error("BypassCache should be true when POST_BYPASS_CACHE=true")
	}
}

func TestLoadInvalidTTLUsesDefault(t *testing.T) {
	t.Setenv("POST_CACHE_TTL", "not-a-duration")
	t.Setenv("POST_TOMBSTONE_TTL", "nope")
	cfg := Load()
	if cfg.CacheTTL != 6*time.Hour {
		t.Errorf("CacheTTL = %s, want 6h", cfg.CacheTTL)
	}
	if cfg.TombstoneTTL != 24*time.Hour {
		t.Errorf("TombstoneTTL = %s, want 24h", cfg.TombstoneTTL)
	}
}
