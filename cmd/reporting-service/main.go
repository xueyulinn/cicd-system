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
	"github.com/xueyulinn/cicd-system/internal/services/reporting"
)

const serviceName = "reporting-service"

func main() {
	ctx := context.Background()

	shutdown, err := observability.Bootstrap(ctx, serviceName)
	if err != nil {
		slog.Error("failed to init observability", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(ctx) }()

	handler, err := reporting.NewHandler()
	if err != nil {
		slog.Error("reporting service failed to initialize", "error", err)
		os.Exit(1)
	}
	defer handler.Close()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	wrapped := observability.HTTPMetricsMiddleware(
		observability.TracingMiddleware(serviceName, mux))

	addr := ":" + config.GetEnvOrDefault("PORT", config.DefaultReportingPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      wrapped,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("service starting", "addr", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("listen failed", "error", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("service shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		slog.Error("forced shutdown", "error", err)
	} else {
		slog.Info("service stopped")
	}
}
