package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("FEED_SERVICE_GRPC_PORT", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("POST_SERVICE_ADDR", "")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

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
	if cfg.MetricsAddr != ":9101" {
		t.Errorf("MetricsAddr = %q, want :9101", cfg.MetricsAddr)
	}
	if cfg.BypassCache {
		t.Error("BypassCache default should be false")
	}
}

func TestLoadOverrides(t *testing.T) {
	t.Setenv("FEED_SERVICE_GRPC_PORT", "9999")

	t.Setenv("FEED_CANDIDATE_POOL_SIZE", "321")
	t.Setenv("FEED_RECENCY_HALF_LIFE", "8h")
	t.Setenv("FEED_BYPASS_CACHE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.Addr() != ":9999" {
		t.Errorf("Addr() = %q, want %q", cfg.Addr(), ":9999")
	}
	if cfg.CandidatePool != 321 || cfg.RecencyHalfLife.String() != "8h0m0s" {
		t.Errorf("numeric overrides not loaded: %+v", cfg)
	}
	if !cfg.BypassCache {
		t.Error("BypassCache should be true when FEED_BYPASS_CACHE=true")
	}
}

func TestLoadRejectsInvalidValues(t *testing.T) {
	t.Setenv("FEED_POST_CACHE_TTL", "never")
	if _, err := Load(); err == nil {
		t.Fatal("Load() error = nil, want invalid duration error")
	}
}
