package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("FEED_SERVICE_GRPC_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("POST_SERVICE_ADDR", "")

	cfg := Load()

	if cfg.GRPCPort != "9091" {
		t.Errorf("GRPCPort = %q, want %q", cfg.GRPCPort, "9091")
	}
	if cfg.Addr() != ":9091" {
		t.Errorf("Addr() = %q, want %q", cfg.Addr(), ":9091")
	}
	if cfg.DefaultPageSize <= 0 || cfg.MaxPageSize < cfg.DefaultPageSize {
		t.Errorf("page size defaults are inconsistent: default=%d max=%d", cfg.DefaultPageSize, cfg.MaxPageSize)
	}
	if cfg.DatabaseURL == "" || cfg.RedisAddr == "" || cfg.PostServiceAddr == "" {
		t.Error("all address fields should have non-empty defaults")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("FEED_SERVICE_GRPC_PORT", "9999")

	cfg := Load()

	if cfg.Addr() != ":9999" {
		t.Errorf("Addr() = %q, want %q", cfg.Addr(), ":9999")
	}
}
