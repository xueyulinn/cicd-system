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
	"github.com/xueyulinn/cicd-system/internal/services/validation"
)

const serviceName = "validation-service"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
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

	handler := validation.NewHandler()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	wrapped := observability.HTTPMetricsMiddleware(
		observability.TracingMiddleware(serviceName, mux))

	addr := ":" + config.GetEnvOrDefault("PORT", config.DefaultValidationPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      wrapped,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	errCh := make(chan error, 1)

	go func() {
		slog.Info("service starting", "service", serviceName, "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("shutdown signal received", "service", serviceName)
	case err := <-errCh:
		slog.Error("listen failed", "service", serviceName, "error", err)
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
