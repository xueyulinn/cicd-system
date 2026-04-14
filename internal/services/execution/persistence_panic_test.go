package execution

import (
	"context"
	"testing"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/store"
)

func TestPersistenceMethodsPanicWithUninitializedStoreInstance(t *testing.T) {
	svc := &Service{store: new(store.Store)}
	ctx := context.Background()

	expectPanic(t, func() {
		_, _, _ = svc.startPipelineRun(ctx, "pipe", runInfo{Branch: "main", Commit: "abc"}, "req-key")
	})

	expectPanic(t, func() {
		_ = svc.finishPipelineRunWithMetrics(ctx, "pipe", 1, store.StatusSuccess, time.Now().Add(-time.Second))
	})

	expectPanic(t, func() {
		_ = svc.finishStageWithMetrics(ctx, "pipe", 1, "build", store.StatusSuccess, time.Now().Add(-time.Second))
	})

	expectPanic(t, func() {
		_ = svc.finishPipelineRun(ctx, "pipe", 1, store.StatusFailed)
	})

	expectPanic(t, func() {
		_ = svc.startStage(ctx, "pipe", 1, "build")
	})

	expectPanic(t, func() {
		_ = svc.finishStage(ctx, "pipe", 1, "build", store.StatusFailed)
	})

	expectPanic(t, func() {
		_ = svc.startJob(ctx, "pipe", 1, "build", "compile", false)
	})

	expectPanic(t, func() {
		_ = svc.finishJob(ctx, "pipe", 1, "build", "compile", store.StatusSuccess)
	})

	expectPanic(t, func() {
		_ = svc.markJobRunning(ctx, "pipe", 1, "build", "compile")
	})
}

func TestServiceReadyPanicsWithUninitializedStoreInstance(t *testing.T) {
	svc := &Service{store: new(store.Store)}
	expectPanic(t, func() {
		_ = svc.Ready(context.Background())
	})
}
