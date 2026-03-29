package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/observability"
	"github.com/CS7580-SEA-SP26/e-team/internal/services/worker"
)

const serviceName = "worker-service"

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdown, err := observability.Init(ctx, serviceName)
	if err != nil {
		slog.Error("failed to init observability", "error", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(ctx) }()

	addr := ":" + config.GetEnvOrDefault("PORT", config.DefaultWorkerPort)

	slog.Info("service starting", "addr", addr)
	if err := worker.Run(ctx, addr); err != nil {
		slog.Error("service error", "error", err)
		os.Exit(1)
	}
}
