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
	"github.com/xueyulinn/cicd-system/internal/services/gateway"
)

const serviceName = "api-gateway"

func main() {
	ctx := context.Background()

	shutdown, err := observability.Init(ctx, serviceName)
	if err != nil {
		slog.Error("failed to init observability", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(ctx) }()

	handler := gateway.NewHandler()

	// HTTP request multiplexer
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	// Register metrics handler provided by Prometheus
	mux.Handle("/metrics", observability.MetricsHandler())

	// wrapped handler with mertrics handler and trace handler
	wrapped := observability.HTTPMetricsMiddleware(
		observability.TracingMiddleware(serviceName, mux))

	addr := ":" + config.GetEnvOrDefault("PORT", config.DefaultGatewayPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      wrapped,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 20 * time.Minute,
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

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("forced shutdown", "error", err)
	} else {
		slog.Info("service stopped")
	}
}
