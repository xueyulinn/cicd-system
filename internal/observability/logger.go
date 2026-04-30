package observability

import (
	"context"
	"log/slog"
	"os"

	"github.com/xueyulinn/cicd-system/internal/models"
	"go.opentelemetry.io/otel/trace"
)

type ctxKeyLogger struct{}

// ContextWithLogger stores a logger in the context.
func ContextWithLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, ctxKeyLogger{}, l)
}

// L retrieves the logger from context, falling back to slog.Default.
func L(ctx context.Context) *slog.Logger {
	if l, ok := ctx.Value(ctxKeyLogger{}).(*slog.Logger); ok {
		return l
	}
	return slog.Default()
}

// WithTraceContext enriches a logger with trace_id and span_id from the current span.
func WithTraceContext(ctx context.Context, l *slog.Logger) *slog.Logger {
	sc := trace.SpanContextFromContext(ctx)
	if !sc.IsValid() {
		return l
	}
	return l.With(
		slog.String("trace_id", sc.TraceID().String()),
		slog.String("span_id", sc.SpanID().String()),
	)
}

// WithPipelineContext enriches a logger with pipeline execution fields.
func WithPipelineContext(l *slog.Logger, pipeline string, runNo int) *slog.Logger {
	return l.With(
		slog.String("pipeline", pipeline),
		slog.Int("run_no", runNo),
	)
}

// WithReportQueryContext enriches a logger with report query fields when present.
func WithReportQueryContext(l *slog.Logger, query models.ReportQuery) *slog.Logger {
	l = l.With(slog.String("pipeline", query.Pipeline))
	if query.Run != nil {
		l = l.With(slog.Int("run_no", *query.Run))
	}
	if query.Stage != "" {
		l = l.With(slog.String("stage", query.Stage))
	}
	if query.Job != "" {
		l = l.With(slog.String("job", query.Job))
	}
	return l
}

func slogLevelFromEnv() slog.Level {
	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG", "debug":
		return slog.LevelDebug
	case "WARN", "warn":
		return slog.LevelWarn
	case "ERROR", "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
