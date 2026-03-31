package observability

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
)

// Tracer returns a named tracer from the global provider.
func Tracer(name string) trace.Tracer {
	return otel.Tracer(name)
}

// InitTracer sets up an OTel TracerProvider.
//
// Environment variables:
//
//	OTEL_EXPORTER_OTLP_ENDPOINT — if set, use OTLP/HTTP exporter.
//	OTEL_TRACES_EXPORTER=stdout — force stdout exporter (useful for local dev).
//
// If neither is configured the provider is still functional (no-op export) so
// spans are created and propagated but not stored.
//
// Returns a shutdown function that must be called on service exit.
func InitTracer(ctx context.Context, serviceName string) (shutdown func(context.Context) error, err error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(semconv.ServiceNameKey.String(serviceName)),
		resource.WithProcessRuntimeDescription(),
		resource.WithHost(),
	)
	if err != nil {
		return nil, err
	}

	var exporter sdktrace.SpanExporter

	switch {
	case os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT") != "":
		exporter, err = otlptracehttp.New(ctx)
		if err != nil {
			return nil, err
		}
	case os.Getenv("OTEL_TRACES_EXPORTER") == "stdout":
		exporter, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
	}

	opts := []sdktrace.TracerProviderOption{
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	}
	if exporter != nil {
		opts = append(opts, sdktrace.WithBatcher(exporter))
	}

	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return tp.Shutdown, nil
}
