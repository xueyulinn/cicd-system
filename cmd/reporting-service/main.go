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
	"github.com/xueyulinn/cicd-system/internal/services/reporting"
)

const serviceName = "reporting-service"

type handler interface {
	RegisterRoutes(mux *http.ServeMux)
	Close()
}

type httpServer interface {
	ListenAndServe() error
	Shutdown(ctx context.Context) error
}

type dependencies struct {
	initObservability     func(ctx context.Context, service string) (func(context.Context) error, error)
	newHandler            func() (handler, error)
	metricsHandler        func() http.Handler
	tracingMiddleware     func(service string, next http.Handler) http.Handler
	httpMetricsMiddleware func(next http.Handler) http.Handler
	getEnvOrDefault       func(key, fallback string) string
	newServer             func(addr string, wrapped http.Handler) httpServer
	notifySignals         func(c chan<- os.Signal, sig ...os.Signal)
}

func defaultDependencies() dependencies {
	return dependencies{
		initObservability: observability.Init,
		newHandler: func() (handler, error) {
			return reporting.NewHandler()
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
		notifySignals: signal.Notify,
	}
}

func run(ctx context.Context, deps dependencies) error {
	shutdown, err := deps.initObservability(ctx, serviceName)
	if err != nil {
		return fmt.Errorf("failed to init observability: %w", err)
	}
	defer func() { _ = shutdown(ctx) }()

	handler, err := deps.newHandler()
	if err != nil {
		return fmt.Errorf("reporting service failed to initialize: %w", err)
	}
	defer handler.Close()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	mux.Handle("/metrics", deps.metricsHandler())

	wrapped := deps.httpMetricsMiddleware(deps.tracingMiddleware(serviceName, mux))

	addr := ":" + deps.getEnvOrDefault("PORT", config.DefaultReportingPort)
	server := deps.newServer(addr, wrapped)
	listenErr := make(chan error, 1)

	go func() {
		slog.Info("service starting", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			listenErr <- err
		}
	}()

	quit := make(chan os.Signal, 1)
	deps.notifySignals(quit, syscall.SIGINT, syscall.SIGTERM)
	select {
	case <-quit:
	case err := <-listenErr:
		return fmt.Errorf("listen failed: %w", err)
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
