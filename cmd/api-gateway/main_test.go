package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"syscall"
	"testing"
)

type fakeRegistrar struct{}

func (fakeRegistrar) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/health", func(http.ResponseWriter, *http.Request) {})
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

func newDeps(srv *fakeServer) dependencies {
	return dependencies{
		initObservability: func(context.Context, string) (func(context.Context) error, error) {
			return func(context.Context) error { return nil }, nil
		},
		newHandler:     func() routeRegistrar { return fakeRegistrar{} },
		metricsHandler: func() http.Handler { return http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}) },
		tracingMiddleware: func(string, http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
		},
		httpMetricsMiddleware: func(h http.Handler) http.Handler { return h },
		getEnvOrDefault:       func(string, string) string { return "8080" },
		newServer:             func(string, http.Handler) httpServer { return srv },
		notifySignals:         func(c chan<- os.Signal, _ ...os.Signal) { c <- syscall.SIGINT },
	}
}

func TestRunInitError(t *testing.T) {
	srv := &fakeServer{}
	deps := newDeps(srv)
	deps.initObservability = func(context.Context, string) (func(context.Context) error, error) {
		return nil, errors.New("init")
	}

	if err := run(context.Background(), deps); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunListenError(t *testing.T) {
	srv := &fakeServer{listenErr: errors.New("listen")}
	deps := newDeps(srv)
	deps.notifySignals = func(chan<- os.Signal, ...os.Signal) {}

	if err := run(context.Background(), deps); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunShutdownError(t *testing.T) {
	srv := &fakeServer{listenErr: http.ErrServerClosed, shutdownErr: errors.New("shutdown")}
	deps := newDeps(srv)

	if err := run(context.Background(), deps); err == nil {
		t.Fatal("expected error")
	}
}

func TestRunGraceful(t *testing.T) {
	srv := &fakeServer{listenErr: http.ErrServerClosed}
	deps := newDeps(srv)

	if err := run(context.Background(), deps); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if srv.shutdownCall != 1 {
		t.Fatalf("shutdown calls = %d, want 1", srv.shutdownCall)
	}
}
