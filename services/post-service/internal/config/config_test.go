package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("POST_SERVICE_GRPC_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("KAFKA_BROKERS", "")

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
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("POST_SERVICE_GRPC_PORT", "9999")

	cfg := Load()

	if cfg.GRPCPort != "9999" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "9999")
	}
	if cfg.Addr() != ":9999" {
		t.Errorf("Addr() = %q, want %q", cfg.Addr(), ":9999")
	}
}
