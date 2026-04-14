package worker

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/moby/moby/client"
)

func TestPingDocker_NilClient(t *testing.T) {
	err := PingDocker(context.Background(), nil)
	if err == nil || !strings.Contains(err.Error(), "docker client not available") {
		t.Fatalf("err=%v", err)
	}
}

func TestNewDockerClient_InvalidHost(t *testing.T) {
	t.Setenv("DOCKER_HOST", "://bad-url")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	cli, err := NewDockerClient(ctx)
	if err == nil {
		t.Fatal("expected error")
	}
	if cli != nil {
		t.Fatalf("cli=%v, want nil", cli)
	}
}

func TestPingDocker_InvalidZeroClient(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	expectPanic(t, func() {
		_ = PingDocker(ctx, &client.Client{})
	})
}
