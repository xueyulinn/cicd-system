package observability

import (
	"net/http"

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

// TracingMiddleware wraps an http.Handler so that every inbound request
// creates a server span and reads incoming trace context headers.
func TracingMiddleware(serviceName string, next http.Handler) http.Handler {
	return otelhttp.NewHandler(next, serviceName,
		otelhttp.WithPropagators(propagation.TraceContext{}),
	)
}
