package worker

import (
	"context"
	"testing"

	"github.com/moby/moby/client"
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

func TestExecuteHelpers_PanicOnNilClient(t *testing.T) {
	expectPanic(t, func() { _ = pullImage(context.Background(), nil, "alpine:latest") })
	expectPanic(t, func() { _, _ = createContainer(context.Background(), nil, "alpine:latest", nil, false) })
	expectPanic(t, func() { _ = startContainer(context.Background(), nil, "id") })
	expectPanic(t, func() { _ = copyWorkspaceToContainer(context.Background(), nil, "id", ".") })
	expectPanic(t, func() { _ = waitContainer(context.Background(), nil, "id") })
	expectPanic(t, func() { _, _ = getLogs(context.Background(), nil, "id") })
	expectPanic(t, func() { _ = removeContainer(context.Background(), nil, "id") })

	_ = (&client.Client{})
}
