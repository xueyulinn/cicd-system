package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"syscall"
	"testing"
)

type fakeHandler struct {
	closed bool
}

func (f *fakeHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", func(http.ResponseWriter, *http.Request) {})
}

func (f *fakeHandler) Close() {
	f.closed = true
}

type fakeServer struct {
	listenErr    error
	shutdownErr  error
	shutdownCall int
}

func (f *fakeServer) ListenAndServe() error {
	return f.listenErr
}

func (f *fakeServer) Shutdown(context.Context) error {
	f.shutdownCall++
	return f.shutdownErr
}

func newDeps(h *fakeHandler, srv *fakeServer) dependencies {
	return dependencies{
		initObservability: func(context.Context, string) (func(context.Context) error, error) {
			return func(context.Context) error { return nil }, nil
		},
		newHandler:     func() handler { return h },
		metricsHandler: func() http.Handler { return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}) },
		tracingMiddleware: func(string, http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		},
		httpMetricsMiddleware: func(h http.Handler) http.Handler { return h },
		getEnvOrDefault:       func(string, string) string { return "8082" },
		newServer:             func(string, http.Handler) httpServer { return srv },
		notifySignals:         func(c chan<- os.Signal, _ ...os.Signal) { c <- syscall.SIGINT },
	}
}

func TestRunInitError(t *testing.T) {
	h := &fakeHandler{}
	srv := &fakeServer{}
	deps := newDeps(h, srv)
	deps.initObservability = func(context.Context, string) (func(context.Context) error, error) {
		return nil, errors.New("init")
	}

	if err := run(context.Background(), deps); err == nil {
		t.Fatal("expected error")
	}
	if h.closed {
		t.Fatal("handler should not be created/closed on init error")
	}
}

func TestRunListenErrorClosesHandler(t *testing.T) {
	h := &fakeHandler{}
	srv := &fakeServer{listenErr: errors.New("listen")}
	deps := newDeps(h, srv)
	deps.notifySignals = func(chan<- os.Signal, ...os.Signal) {}

	if err := run(context.Background(), deps); err == nil {
		t.Fatal("expected error")
	}
	if !h.closed {
		t.Fatal("expected handler close")
	}
}

func TestRunShutdownError(t *testing.T) {
	h := &fakeHandler{}
	srv := &fakeServer{listenErr: http.ErrServerClosed, shutdownErr: errors.New("shutdown")}
	deps := newDeps(h, srv)

	if err := run(context.Background(), deps); err == nil {
		t.Fatal("expected error")
	}
	if !h.closed {
		t.Fatal("expected handler close")
	}
}

func TestRunGraceful(t *testing.T) {
	h := &fakeHandler{}
	srv := &fakeServer{listenErr: http.ErrServerClosed}
	deps := newDeps(h, srv)

	if err := run(context.Background(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !h.closed {
		t.Fatal("expected handler close")
	}
}
