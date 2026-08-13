package config

import (
	"testing"
	"time"
)

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
	if len(cfg.KafkaBrokers) == 0 || cfg.DatabaseURL == "" || cfg.RedisAddr == "" {
		t.Error("all address fields should have non-empty defaults")
	}
	if cfg.PostTopic != "post-events" || cfg.FollowTopic != "follow-events" ||
		cfg.PostDLQTopic != "post-events.dlq" || cfg.FollowDLQTopic != "follow-events.dlq" {
		t.Errorf("unexpected topic defaults: %+v", cfg)
	}
	if cfg.FanoutBatchSize != 500 || cfg.MaxRetries != 3 ||
		cfg.RetryBackoff != 250*time.Millisecond || cfg.FollowerCountCacheTTL != time.Minute ||
		cfg.TombstoneTTL != 24*time.Hour {
		t.Errorf("unexpected processing defaults: %+v", cfg)
	}
}

func TestLoadOverridesAndInvalidIntFallsBack(t *testing.T) {
	t.Setenv("CELEBRITY_FOLLOWER_THRESHOLD", "500")
	t.Setenv("MAX_TIMELINE_LEN", "not-a-number")
	t.Setenv("KAFKA_BROKERS", " kafka-a:9092, kafka-b:9092 ")
	t.Setenv("RETRY_BACKOFF", "2s")

	cfg := Load()

	if cfg.CelebrityFollowerThreshold != 500 {
		t.Errorf("CelebrityFollowerThreshold = %d, want %d", cfg.CelebrityFollowerThreshold, 500)
	}
	if cfg.MaxTimelineLen != 1_000 {
		t.Errorf("MaxTimelineLen should fall back to default on parse error, got %d", cfg.MaxTimelineLen)
	}
	if len(cfg.KafkaBrokers) != 2 || cfg.KafkaBrokers[1] != "kafka-b:9092" {
		t.Errorf("KafkaBrokers = %#v", cfg.KafkaBrokers)
	}
	if cfg.RetryBackoff != 2*time.Second {
		t.Errorf("RetryBackoff = %s", cfg.RetryBackoff)
	}
}
