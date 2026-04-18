package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/xueyulinn/cicd-system/internal/config"
	"github.com/xueyulinn/cicd-system/internal/observability"
	"github.com/xueyulinn/cicd-system/internal/services/worker"
)

const serviceName = "worker-service"

type handler interface {
	RegisterRoutes(mux *http.ServeMux)
	Run(ctx context.Context) error
	Close()
}

type httpServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type dependencies struct {
	notifyContext         func(parent context.Context, signals ...os.Signal) (context.Context, context.CancelFunc)
	initObservability     func(ctx context.Context, service string) (func(context.Context) error, error)
	newHandler            func() handler
	metricsHandler        func() http.Handler
	tracingMiddleware     func(service string, next http.Handler) http.Handler
	httpMetricsMiddleware func(next http.Handler) http.Handler
	getEnvOrDefault       func(key, fallback string) string
	newServer             func(addr string, wrapped http.Handler) httpServer
}

func defaultDependencies() dependencies {
	return dependencies{
		notifyContext:     signal.NotifyContext,
		initObservability: observability.Init,
		newHandler: func() handler {
			return worker.NewHandler()
		},
		metricsHandler:        observability.MetricsHandler,
		tracingMiddleware:     observability.TracingMiddleware,
		httpMetricsMiddleware: observability.HTTPMetricsMiddleware,
		getEnvOrDefault:       config.GetEnvOrDefault,
		newServer: func(addr string, wrapped http.Handler) httpServer {
			return &http.Server{
				Addr:         addr,
				Handler:      wrapped,
				ReadTimeout:  15 * time.Second,
				WriteTimeout: 15 * time.Second,
				IdleTimeout:  60 * time.Second,
			}
		},
	}
}

func run(parent context.Context, deps dependencies) error {
	ctx, stop := deps.notifyContext(parent, os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdown, err := deps.initObservability(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("failed to init observability: %w", err)
	}
	defer func() { _ = shutdown(ctx) }()

	handler := deps.newHandler()
	defer handler.Close()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.Handle("/metrics", deps.metricsHandler())

	wrapped := deps.httpMetricsMiddleware(deps.tracingMiddleware(serviceName, mux))

	addr := ":" + deps.getEnvOrDefault("PORT", config.DefaultWorkerPort)
	server := deps.newServer(addr, wrapped)

	errCh := make(chan error, 2)

	go func() {
		slog.Info("service starting", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	go func() {
		if err := handler.Run(ctx); err != nil && ctx.Err() == nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		slog.Error("service error", "error", err)
		stop()
	case <-ctx.Done():
	}

	slog.Info("service shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("forced shutdown: %w", err)
	}
	slog.Info("service stopped")
	return nil
}

func main() {
	if err := run(context.Background(), defaultDependencies()); err != nil {
		slog.Error("service exited with error", "error", err)
		os.Exit(1)
	}
}
