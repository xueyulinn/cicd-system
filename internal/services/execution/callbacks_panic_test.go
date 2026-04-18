package execution

import (
	"context"
	"testing"

	"github.com/xueyulinn/cicd-system/internal/api"
	"github.com/xueyulinn/cicd-system/internal/store"
)

func expectPanic(t *testing.T, fn func()) {
	t.Helper()
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic")
		}
	}()
	fn()
}

func TestHandleJobStartedPanicsWhenStoreIsUninitialized(t *testing.T) {
	svc := &Service{store: new(store.Store)}
	expectPanic(t, func() {
		_ = svc.HandleJobStarted(context.Background(), api.JobStatusCallbackRequest{
			Pipeline: "p",
			RunNo:    1,
			Stage:    "build",
			Job:      "compile",
			Status:   "started",
		})
	})
}

func TestHandleJobFinishedPanicsWhenStoreIsUninitialized(t *testing.T) {
	svc := &Service{store: new(store.Store)}
	expectPanic(t, func() {
		_ = svc.HandleJobFinished(context.Background(), api.JobStatusCallbackRequest{
			Pipeline: "p",
			RunNo:    1,
			Stage:    "build",
			Job:      "compile",
			Status:   store.StatusSuccess,
		})
	})
}
