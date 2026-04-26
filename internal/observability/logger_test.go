package observability

import (
	"context"
	"io"
	"log/slog"
	"testing"
)

func TestSlogLevelFromEnv_debug(t *testing.T) {
	t.Setenv("LOG_LEVEL", "DEBUG")
	if got := slogLevelFromEnv(); got != slog.LevelDebug {
		t.Fatalf("got %v want DEBUG", got)
	}
}

func TestContextWithLogger_roundTrip(t *testing.T) {
	base := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx := ContextWithLogger(context.Background(), base)
	if L(ctx) != base {
		t.Fatal("L(ctx) should return stored logger")
	}
	if L(context.Background()) == base {
		t.Fatal("empty ctx should not return injected logger")
	}
}

func TestWithPipelineContext(t *testing.T) {
	l := slog.New(slog.NewJSONHandler(io.Discard, nil))
	out := WithPipelineContext(l, "pipe-a", 7)
	// Ensure With was applied (no panic); child should carry attrs when logging.
	_ = out
}

func TestWithTraceContext_invalidSpan_returnsSame(t *testing.T) {
	l := slog.New(slog.NewJSONHandler(io.Discard, nil))
	out := WithTraceContext(context.Background(), l)
	if out != l {
		t.Fatal("without valid span, expect same logger pointer")
	}
}
