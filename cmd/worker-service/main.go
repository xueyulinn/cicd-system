package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/CS7580-SEA-SP26/e-team/internal/services/worker"
)

func main() {
	addr := ":8003"
	if a := os.Getenv("PORT"); a != "" {
		addr = ":" + a
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	fmt.Printf("Worker service listening on %s\n", addr)
	if err := worker.Run(ctx, addr); err != nil {
		fmt.Fprintf(os.Stderr, "Worker service error: %v\n", err)
		os.Exit(1)
	}
}
