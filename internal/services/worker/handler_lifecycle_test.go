package worker

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/mq"
)

func TestNewHandler_InitializationFailure(t *testing.T) {
	t.Setenv("DOCKER_HOST", "://bad-url")
	h := NewHandler()
	if h == nil {
		t.Fatal("handler is nil")
	}
	if h.initErr == nil {
		t.Fatal("expected initErr")
	}
}

func TestHandlerClose_NilSafe(t *testing.T) {
	var h *Handler
	h.Close()
}

func TestHandlerRun_Branches(t *testing.T) {
	var nilHandler *Handler
	if err := nilHandler.Run(context.Background()); err != nil {
		t.Fatalf("nil handler run err=%v", err)
	}

	initErr := errors.New("init failed")
	h := &Handler{initErr: initErr}
	if err := h.Run(context.Background()); !errors.Is(err, initErr) {
		t.Fatalf("err=%v", err)
	}

	h = &Handler{service: nil}
	if err := h.Run(context.Background()); err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}

	h = &Handler{service: &Service{docker: nil, jobConsumers: []mq.Consumer{&fakeConsumer{}}, jobTimeout: time.Second}}
	err := h.Run(context.Background())
	if err == nil || !strings.Contains(err.Error(), "docker client not available") {
		t.Fatalf("err=%v", err)
	}
}
