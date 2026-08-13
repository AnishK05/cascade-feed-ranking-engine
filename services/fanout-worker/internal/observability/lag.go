package observability

import (
	"context"
	"log/slog"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

// WatchLag periodically records the consumer group's lag per topic/partition.
func WatchLag(ctx context.Context, client *kgo.Client, group string, metrics *Metrics, logger *slog.Logger) {
	if client == nil || metrics == nil {
		return
	}
	if logger == nil {
		logger = slog.Default()
	}
	admin := kadm.NewClient(client)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	for {
		recordLag(ctx, admin, group, metrics, logger)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func recordLag(ctx context.Context, admin *kadm.Client, group string, metrics *Metrics, logger *slog.Logger) {
	queryCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	lags, err := admin.Lag(queryCtx, group)
	if err != nil {
		logger.Warn("read kafka consumer lag", "error", err)
		return
	}
	described, ok := lags[group]
	if !ok {
		return
	}
	if err := described.Error(); err != nil {
		logger.Warn("kafka group lag fetch", "error", err)
		return
	}
	for _, lag := range described.Lag.Sorted() {
		if lag.Err != nil {
			continue
		}
		metrics.SetKafkaLag(lag.Topic, lag.Partition, lag.Lag)
	}
}
