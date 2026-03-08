package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/config"
	"github.com/CS7580-SEA-SP26/e-team/internal/services/reporting"
)

func main() {
	handler, err := reporting.NewHandler()
	if err != nil {
		log.Fatalf("Reporting service failed to initialize: %v", err)
	}
	defer handler.Close()

	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	addr := ":" + config.GetEnvOrDefault("PORT", config.DefaultReportingPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("Reporting service starting on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Reporting service failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Reporting service shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Reporting service forced shutdown: %v", err)
	} else {
		log.Println("Reporting service stopped")
	}
}
