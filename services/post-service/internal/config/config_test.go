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
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("POST_SERVICE_GRPC_PORT", "9999")
	t.Setenv("KAFKA_BROKERS", " kafka-1:9092, kafka-2:9092 ,,")
	t.Setenv("KAFKA_TOPIC", "custom-posts")
	t.Setenv("POST_CACHE_TTL", "45m")

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
	if cfg.KafkaTopic != "custom-posts" || cfg.CacheTTL != 45*time.Minute {
		t.Errorf("topic/TTL = %q/%s", cfg.KafkaTopic, cfg.CacheTTL)
	}
}

func TestLoadInvalidTTLUsesDefault(t *testing.T) {
	t.Setenv("POST_CACHE_TTL", "not-a-duration")
	if got := Load().CacheTTL; got != 6*time.Hour {
		t.Errorf("CacheTTL = %s, want 6h", got)
	}
}
