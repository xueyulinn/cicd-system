package observability

import (
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/propagation"
)

// NewHTTPClient returns an *http.Client that automatically propagates
// W3C trace context (traceparent / tracestate) on outbound requests.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Transport: otelhttp.NewTransport(http.DefaultTransport),
	}
}

// NewInstrumentedHTTPClient returns an *http.Client that propagates trace context and records
// Prometheus metrics (http_client_request_duration_seconds, http_client_requests_total) for each request.
// client is the local service name (e.g. api-gateway); upstream is the logical downstream (e.g. validation).
func NewInstrumentedHTTPClient(client, upstream string, timeout time.Duration) *http.Client {
	base := otelhttp.NewTransport(http.DefaultTransport)
	return &http.Client{
		Transport: newMetricsRoundTripper(client, upstream, base),
		Timeout:   timeout,
	}
}

type metricsRoundTripper struct {
	next     http.RoundTripper
	client   string
	upstream string
}

func newMetricsRoundTripper(client, upstream string, next http.RoundTripper) http.RoundTripper {
	return &metricsRoundTripper{next: next, client: client, upstream: upstream}
}

func (t *metricsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := t.next.RoundTrip(req)
	elapsed := time.Since(start).Seconds()

	code := "error"
	if err == nil && resp != nil {
		code = strconv.Itoa(resp.StatusCode)
	}

	httpClientRequestsTotal.WithLabelValues(t.client, t.upstream, code).Inc()
	httpClientRequestDurationSeconds.WithLabelValues(t.client, t.upstream).Observe(elapsed)
	return resp, err
}

// TracingMiddleware wraps an http.Handler so that every inbound request
// creates a server span and reads incoming trace context headers.
func TracingMiddleware(serviceName string, next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, serviceName,
		otelhttp.WithPropagators(propagation.TraceContext{}),
	)
}
