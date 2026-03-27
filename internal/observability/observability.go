// Package observability provides a unified setup for structured logging,
// Prometheus metrics, and OpenTelemetry tracing across all CI/CD services.
//
// Usage (in each service's main.go):
//
//	shutdown, err := observability.Init(ctx, "api-gateway")
//	if err != nil { log.Fatal(err) }
//	defer shutdown(ctx)
//
//	mux := http.NewServeMux()
//	handler.RegisterRoutes(mux)
//	mux.Handle("/metrics", observability.MetricsHandler())
//	wrapped := observability.HTTPMetricsMiddleware(observability.TracingMiddleware("api-gateway", mux))
//
// Environment variables:
//
//	LOG_LEVEL                     — DEBUG | INFO (default) | WARN | ERROR
//	OTEL_EXPORTER_OTLP_ENDPOINT  — e.g. http://otel-collector:4318 (enables OTLP export)
//	OTEL_TRACES_EXPORTER=stdout   — pretty-print spans to stdout (local dev)
package observability

import (
	"context"
	"log/slog"
)

// Init sets up logging, metrics, and tracing for a service.
// Returns a shutdown function that flushes pending traces.
func Init(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	logger := NewLogger(serviceName)
	slog.SetDefault(logger)

	RegisterMetrics()

	tracerShutdown, err := InitTracer(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	logger.Info("observability initialized",
		slog.String("service", serviceName),
	)

	return tracerShutdown, nil
}
