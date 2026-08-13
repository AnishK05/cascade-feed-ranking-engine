// Package observability exposes Prometheus metrics, a /metrics HTTP server, and a gRPC
// interceptor that propagates request IDs from Gateway metadata into structured logs.
package observability

import (
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics records Feed Service request and cache-hydration counters.
type Metrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	hits     prometheus.Counter
	misses   prometheus.Counter
}

func NewMetrics() *Metrics {
	return &Metrics{
		requests: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "feed_requests_total",
			Help: "GetFeed RPCs handled by Feed Service.",
		}, []string{"cache_hit"}),
		duration: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "feed_request_duration_seconds",
			Help:    "GetFeed latency, labeled by whether every candidate hydrated from Redis.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		}, []string{"cache_hit"}),
		hits: promauto.NewCounter(prometheus.CounterOpts{
			Name: "feed_cache_hits_total",
			Help: "Post IDs hydrated from Redis without calling Post Service.",
		}),
		misses: promauto.NewCounter(prometheus.CounterOpts{
			Name: "feed_cache_misses_total",
			Help: "Post IDs missing from Redis and fetched from Post Service.",
		}),
	}
}

func (m *Metrics) ObserveGetFeed(duration time.Duration, hits, misses int) {
	if m == nil {
		return
	}
	hit := misses == 0
	label := strconv.FormatBool(hit)
	m.requests.WithLabelValues(label).Inc()
	m.duration.WithLabelValues(label).Observe(duration.Seconds())
	if hits > 0 {
		m.hits.Add(float64(hits))
	}
	if misses > 0 {
		m.misses.Add(float64(misses))
	}
}
