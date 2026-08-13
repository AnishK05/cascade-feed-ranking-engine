package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"time"

	"github.com/AnishK05/cascade-feed-ranking-engine/services/feed-service/internal/feedserver"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const requestIDHeader = "x-request-id"

var (
	grpcHandled = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "grpc_server_handled_total",
		Help: "Total gRPC requests handled, labeled by method and status code.",
	}, []string{"grpc_service", "grpc_method", "grpc_code"})
	grpcDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "grpc_server_duration_seconds",
		Help:    "gRPC handler latency.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
	}, []string{"grpc_service", "grpc_method"})
)

// UnaryServerInterceptor extracts x-request-id from incoming metadata, records standard
// gRPC counters/histograms, and logs each RPC with the request ID.
func UnaryServerInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	if logger == nil {
		logger = slog.Default()
	}
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		requestID := incomingRequestID(ctx)
		ctx = feedserver.WithRequestID(ctx, requestID)
		started := time.Now()
		resp, err := handler(ctx, req)
		service, method := splitMethod(info.FullMethod)
		code := status.Code(err)
		grpcHandled.WithLabelValues(service, method, code.String()).Inc()
		grpcDuration.WithLabelValues(service, method).Observe(time.Since(started).Seconds())
		logger.InfoContext(ctx, "grpc request",
			"grpc_service", service, "grpc_method", method,
			"grpc_code", code.String(), "duration_ms", time.Since(started).Milliseconds(),
			"request_id", requestID,
		)
		if err != nil && code == codes.Unknown {
			logger.ErrorContext(ctx, "grpc handler error", "error", err, "request_id", requestID)
		}
		return resp, err
	}
}

func incomingRequestID(ctx context.Context) string {
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if values := md.Get(requestIDHeader); len(values) > 0 && values[0] != "" {
			return values[0]
		}
	}
	return newRequestID()
}

func newRequestID() string {
	var buf [16]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "missing-request-id"
	}
	return hex.EncodeToString(buf[:])
}

func splitMethod(fullMethod string) (service, method string) {
	service, method = "unknown", "unknown"
	if fullMethod == "" {
		return service, method
	}
	trimmed := fullMethod
	if trimmed[0] == '/' {
		trimmed = trimmed[1:]
	}
	for i := len(trimmed) - 1; i >= 0; i-- {
		if trimmed[i] == '/' {
			return trimmed[:i], trimmed[i+1:]
		}
	}
	return trimmed, method
}
