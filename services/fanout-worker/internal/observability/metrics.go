// Package observability exposes Prometheus metrics for the Fanout Worker, including
// processed-event counts, fanout lag, and Kafka consumer lag.
package observability

import (
	"context"
	"strconv"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/consumer"
	"github.com/AnishK05/cascade-feed-ranking-engine/services/fanout-worker/internal/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	processed *prometheus.CounterVec
	lag       prometheus.Histogram
	kafkaLag  *prometheus.GaugeVec
}

func NewMetrics() *Metrics {
	return &Metrics{
		processed: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "fanout_events_processed_total",
			Help: "Kafka records the Fanout Worker finished processing, labeled by topic and result.",
		}, []string{"topic", "result"}),
		lag: promauto.NewHistogram(prometheus.HistogramOpts{
			Name:    "fanout_lag_ms",
			Help:    "Milliseconds between event creation and successful processing.",
			Buckets: []float64{10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000, 30000},
		}),
		kafkaLag: promauto.NewGaugeVec(prometheus.GaugeOpts{
			Name: "kafka_consumer_lag",
			Help: "Fanout Worker consumer group lag per topic and partition.",
		}, []string{"topic", "partition"}),
	}
}

type instrumentedProcessor struct {
	inner   consumer.Processor
	metrics *Metrics
}

func Instrument(inner consumer.Processor, metrics *Metrics) consumer.Processor {
	if metrics == nil {
		return inner
	}
	return &instrumentedProcessor{inner: inner, metrics: metrics}
}

func (p *instrumentedProcessor) Process(ctx context.Context, topic string, payload []byte) error {
	err := p.inner.Process(ctx, topic, payload)
	result := "ok"
	if err != nil {
		result = "error"
	}
	p.metrics.processed.WithLabelValues(topic, result).Inc()
	if err == nil {
		if created := eventCreatedAtUnixMs(topic, payload); created > 0 {
			lag := float64(time.Now().UnixMilli() - created)
			if lag < 0 {
				lag = 0
			}
			p.metrics.lag.Observe(lag)
		}
	}
	return err
}

func (m *Metrics) SetKafkaLag(topic string, partition int32, lag int64) {
	if m == nil {
		return
	}
	m.kafkaLag.WithLabelValues(topic, strconv.FormatInt(int64(partition), 10)).Set(float64(lag))
}

func eventCreatedAtUnixMs(_ string, payload []byte) int64 {
	if event, err := events.ParsePost(payload); err == nil {
		switch event := event.(type) {
		case events.PostCreated:
			return event.CreatedAtUnixMs
		case events.PostDeleted:
			return event.DeletedAtUnixMs
		}
	}
	if event, err := events.ParseFollow(payload); err == nil {
		switch event := event.(type) {
		case events.FollowCreated:
			return event.CreatedAtUnixMs
		case events.FollowDeleted:
			return event.DeletedAtUnixMs
		}
	}
	return 0
}
