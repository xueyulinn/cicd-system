package observability

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// NewInstrumentedHTTPClient returns an *http.Client that propagates trace context and records
// Prometheus metrics (http_client_request_duration_seconds, http_client_requests_total) for each request.
// client is the local service name (e.g. api-gateway); downstream is the logical downstream (e.g. validation).
func NewInstrumentedHTTPClient(clientName string, downstream string, timeout time.Duration) *http.Client {
	base := otelhttp.NewTransport(http.DefaultTransport)
	return &http.Client{
		Transport: newMetricsRoundTripper(clientName, downstream, base),
		Timeout:   timeout,
	}
}

type metricsRoundTripper struct {
	next     http.RoundTripper
	clientName   string
	downstream string
}

func newMetricsRoundTripper(clientName string, downstream string, next http.RoundTripper) http.RoundTripper {
	return &metricsRoundTripper{next: next, clientName: clientName, downstream: downstream}
}

// Overrides RoundTrip func to implement RoundTripper
func (t *metricsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.next.RoundTrip(req)
	elapsed := time.Since(start).Seconds()

	code := "error"
	if err == nil && resp != nil {
		code = strconv.Itoa(resp.StatusCode)
	}

	httpClientRequestsTotal.WithLabelValues(t.clientName, t.downstream, code).Inc()
	httpClientRequestDurationSeconds.WithLabelValues(t.clientName, t.downstream).Observe(elapsed)
	return resp, err
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
