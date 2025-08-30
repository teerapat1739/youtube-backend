package server

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// StartServerWithGracefulShutdown starts the server and handles graceful shutdown
func StartServerWithGracefulShutdown(server *http.Server, port string) {
	// Channel to listen for interrupt signals
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server in a goroutine
	go func() {
		log.Printf("🚀 Server starting on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ Server failed to start: %v", err)
		}
	}()

	// Wait for interrupt signal
	<-stop
	log.Println("🚑 Shutdown signal received")

	// Create a context with timeout for shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	log.Println("🔄 Shutting down server gracefully...")
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("⚠️  Server shutdown error: %v", err)
	} else {
		log.Println("✅ Server stopped gracefully")
	}
}