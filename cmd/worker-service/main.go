package main

import (
	"context"
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

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdown, err := observability.Bootstrap(ctx, serviceName)
	if err != nil {
		slog.Error("failed to init observability", "service", serviceName, "error", err)
		os.Exit(1)
	}
	defer func() {
		obsShutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := shutdown(obsShutdownCtx); err != nil {
			slog.Error("observability shutdown failed", "service", serviceName, "error", err)
		}
	}()

	handler := worker.NewHandler()
	defer handler.Close()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	wrapped := observability.HTTPMetricsMiddleware(
		observability.TracingMiddleware(serviceName, mux))

	addr := ":" + config.GetEnvOrDefault("PORT", config.DefaultWorkerPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      wrapped,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	errCh := make(chan error, 2)

	go func() {
		slog.Info("service starting", "service", serviceName, "addr", addr)
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
		slog.Error("service error", "service", serviceName, "error", err)
		stop()
	case <-ctx.Done():
		slog.Info("shutdown signal received", "service", serviceName)
	}

	slog.Info("service shutting down", "service", serviceName)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "service", serviceName, "error", err)
	} else {
		slog.Info("service stopped", "service", serviceName)
	}
}
