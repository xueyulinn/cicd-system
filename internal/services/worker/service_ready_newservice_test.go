package worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
	"github.com/moby/moby/client"
)

func newPingDockerClient(t *testing.T) (*client.Client, func()) {
	t.Helper()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "_ping") {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("OK"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))

	cli, err := client.New(
		client.WithHost(srv.URL),
		client.WithVersion("1.53"),
		client.WithHTTPClient(srv.Client()),
	)
	if err != nil {
		srv.Close()
		t.Fatalf("create docker client: %v", err)
	}
	return cli, func() {
		_ = cli.Close()
		srv.Close()
	}
}

func TestServiceReady_RabbitMQConfigErrorAfterDockerReady(t *testing.T) {
	dockerCli, cleanup := newPingDockerClient(t)
	defer cleanup()

	svc := &Service{
		docker:   dockerCli,
		mqConfig: mq.Config{URL: "://bad-url", JobQueue: "jobs"},
	}

	err := svc.Ready(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rabbitmq not ready") {
		t.Fatalf("err=%v", err)
	}
}

func TestNewService_DockerInitError(t *testing.T) {
	t.Setenv("DOCKER_HOST", "://bad-url")
	_, err := NewService(context.Background(), 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "create docker client") {
		t.Fatalf("err=%v", err)
	}
}
