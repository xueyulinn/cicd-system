package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"testing"
)

type fakeHandler struct {
	run       func(context.Context) error
	closed    bool
	registerd bool
}

func (f *fakeHandler) RegisterRoutes(mux *http.ServeMux) {
	f.registerd = true
	mux.HandleFunc("/health", func(http.ResponseWriter, *http.Request) {})
}

func (f *fakeHandler) Run(ctx context.Context) error {
	if f.run != nil {
		return f.run(ctx)
	}
	return nil
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
		notifyContext: func(parent context.Context, _ ...os.Signal) (context.Context, context.CancelFunc) {
			return context.WithCancel(parent)
		},
		initObservability: func(context.Context, string) (func(context.Context) error, error) {
			return func(context.Context) error { return nil }, nil
		},
		newHandler:     func() handler { return h },
		metricsHandler: func() http.Handler { return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}) },
		tracingMiddleware: func(string, http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		},
		httpMetricsMiddleware: func(h http.Handler) http.Handler { return h },
		getEnvOrDefault:       func(string, string) string { return "8084" },
		newServer:             func(string, http.Handler) httpServer { return srv },
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
}

func TestRunWorkerErrorTriggersShutdown(t *testing.T) {
	h := &fakeHandler{
		run: func(context.Context) error { return errors.New("worker failed") },
	}
	srv := &fakeServer{listenErr: http.ErrServerClosed}
	deps := newDeps(h, srv)

	if err := run(context.Background(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !h.closed {
		t.Fatal("expected handler close")
	}
	if srv.shutdownCall != 1 {
		t.Fatalf("shutdown calls = %d, want 1", srv.shutdownCall)
	}
}

func TestRunListenErrorTriggersShutdown(t *testing.T) {
	h := &fakeHandler{
		run: func(ctx context.Context) error {
			<-ctx.Done()
			return nil
		},
	}
	srv := &fakeServer{listenErr: errors.New("listen")}
	deps := newDeps(h, srv)

	if err := run(context.Background(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv.shutdownCall != 1 {
		t.Fatalf("shutdown calls = %d, want 1", srv.shutdownCall)
	}
}

func TestRunShutdownError(t *testing.T) {
	h := &fakeHandler{
		run: func(context.Context) error { return errors.New("worker failed") },
	}
	srv := &fakeServer{
		listenErr:   http.ErrServerClosed,
		shutdownErr: errors.New("shutdown"),
	}
	deps := newDeps(h, srv)

	if err := run(context.Background(), deps); err == nil {
		t.Fatal("expected error")
	}
}
