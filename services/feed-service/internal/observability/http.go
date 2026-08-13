package observability

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// ServeMetrics starts a background HTTP server that exposes /metrics for Prometheus.
func ServeMetrics(addr string, logger *slog.Logger) *http.Server {
	if logger == nil {
		logger = slog.Default()
	}
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	server := &http.Server{Addr: addr, Handler: mux}
	go func() {
		logger.Info("metrics listening", "address", addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("metrics HTTP server stopped", "error", err)
		}
	}()
	return server
}
