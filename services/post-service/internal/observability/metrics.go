package observability

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var postgresQueries = promauto.NewCounterVec(prometheus.CounterOpts{
	Name: "post_postgres_queries_total",
	Help: "PostgreSQL queries issued by Post Service, labeled by operation.",
}, []string{"op"})

// RecordPostgresQuery increments the comparison counter used by the Phase 12
// before/after cache benchmark (IMPLEMENTATION_PLAN.md §13.3).
func RecordPostgresQuery(op string) {
	postgresQueries.WithLabelValues(op).Inc()
}
