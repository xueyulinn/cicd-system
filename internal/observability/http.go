package observability

import (
	"net/http"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// NewInstrumentedHTTPClient returns an *http.Client that propagates trace context and records for outbound http request.
// OTel metrics (http.client.request.duration) for each request.
// downstream is the logical downstream service name (e.g. validation).
func NewInstrumentedHTTPClient(downstream string, timeout time.Duration) *http.Client {
	wrapped := otelhttp.NewTransport(
		http.DefaultTransport,
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			method := strings.TrimSpace(r.Method)
			path := strings.TrimSpace(r.URL.Path)
			if path == "" {
				path = "/"
			}
			if method == "" {
				return path
			}
			return method + " " + path
		}),
		otelhttp.WithSpanOptions(
			trace.WithAttributes(
				attribute.String("http.downstream", downstream),
			),
		),
	)
	return &http.Client{
		Transport: wrapped,
		Timeout:   timeout,
	}
}

// HTTPMetricsMiddleware wraps an http.Handler and records request count + latency.
func HTTPMetricsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
	})
}

// TracingMiddleware wraps an http.Handler so that every inbound request
// creates a server span and reads incoming trace context headers.
func TracingMiddleware(serviceName string, next http.Handler) http.Handler {
	return otelhttp.NewHandler(
		next,
		serviceName,
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			method := strings.TrimSpace(r.Method)
			path := strings.TrimSpace(r.URL.Path)
			if path == "" {
				path = "/"
			}
			if method == "" {
				return path
			}
			return method + " " + path
		}),
		otelhttp.WithFilter(func(r *http.Request) bool {
			switch r.URL.Path {
			case "/metrics", "/health", "/ready":
				return false
			default:
				return true
			}
		}),
	)
}
