package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"workspace/internal/polling"
)

func main() {
	// Load configuration
	cfg := polling.NewConfig()
	log.Printf("Loaded polling configuration: interval=%v", cfg.Interval)

	// Create poller
	poller := polling.NewPoller(cfg)

	// Set up signal handling for graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Start the poller in a goroutine
	go poller.Start(ctx)

	// Wait for shutdown signal
	<-sigChan
	log.Println("Shutdown signal received")
	cancel()

	// Give the poller a moment to clean up
	time.Sleep(100 * time.Millisecond)
	log.Println("Application shutdown complete")
}
