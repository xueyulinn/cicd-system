package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/CS7580-SEA-SP26/e-team/internal/services/execution"
)

func main() {
	// Create validation handler
	handler := execution.NewHandler()
	
	// Create HTTP server
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)
	
	server := &http.Server{
		Addr:         ":8002",
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 20 * time.Minute, // pipeline run can take many minutes
		IdleTimeout:  60 * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Printf("Execution service starting on port 8002")
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Execution service failed: %v", err)
		}
	}()

// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)	
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Execution service shutting down...")

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Execution service forced shutdown: %v", err)
	} else {
		log.Println("Execution service stopped")
	}
}
