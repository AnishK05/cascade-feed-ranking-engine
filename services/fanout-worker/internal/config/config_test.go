package config

import "testing"

func TestLoadDefaults(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "")
	t.Setenv("DATABASE_URL", "")
	t.Setenv("REDIS_ADDR", "")
	t.Setenv("CELEBRITY_FOLLOWER_THRESHOLD", "")
	t.Setenv("MAX_TIMELINE_LEN", "")
	t.Setenv("BACKFILL_COUNT", "")

	cfg := Load()

	if cfg.CelebrityFollowerThreshold != 10_000 {
		t.Errorf("CelebrityFollowerThreshold = %d, want %d", cfg.CelebrityFollowerThreshold, 10_000)
	}
	if cfg.MaxTimelineLen != 1_000 {
		t.Errorf("MaxTimelineLen = %d, want %d", cfg.MaxTimelineLen, 1_000)
	}
	if cfg.BackfillCount != 20 {
		t.Errorf("BackfillCount = %d, want %d", cfg.BackfillCount, 20)
	}
	if cfg.KafkaBrokers == "" || cfg.DatabaseURL == "" || cfg.RedisAddr == "" {
		t.Error("all address fields should have non-empty defaults")
	}
}

func TestLoadOverridesAndInvalidIntFallsBack(t *testing.T) {
	t.Setenv("CELEBRITY_FOLLOWER_THRESHOLD", "500")
	t.Setenv("MAX_TIMELINE_LEN", "not-a-number")

	cfg := Load()

	if cfg.CelebrityFollowerThreshold != 500 {
		t.Errorf("CelebrityFollowerThreshold = %d, want %d", cfg.CelebrityFollowerThreshold, 500)
	}
	if cfg.MaxTimelineLen != 1_000 {
		t.Errorf("MaxTimelineLen should fall back to default on parse error, got %d", cfg.MaxTimelineLen)
	}
}
