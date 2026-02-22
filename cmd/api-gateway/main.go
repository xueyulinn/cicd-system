package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/services/gateway"
)

func main() {
	// Create gateway handler
	handler := gateway.NewHandler()

	// Create HTTP server
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	addr := ":" + getEnvOrDefault("PORT", "8000")
	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		// run requests can take several minutes while execution service completes jobs
		WriteTimeout: 20 * time.Minute,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("API Gateway starting on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("API Gateway failed: %v", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("API Gateway shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("API Gateway forced shutdown: %v", err)
	} else {
		log.Println("API Gateway stopped")
	}
}

func getEnvOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
