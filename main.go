package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"workspace/internal/metrics"
)

func main() {
	// Initialize metrics
	m := metrics.NewMetrics()

	// Set up HTTP server
	mux := http.NewServeMux()

	// Add metrics endpoint
	mux.Handle("/metrics", m.Handler())

	// Add a sample endpoint to demonstrate metrics
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Hello, World!"))
	})

	// Add a health check endpoint
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Wrap with metrics middleware
	handler := m.RequestTrackingMiddleware(mux)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	// Start a background goroutine to update system metrics
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for range ticker.C {
			// In a real implementation, you would get actual system metrics
			// For now, we'll use dummy values
			m.UpdateSystemMetrics(1024*1024, 0.1, 5)
		}
	}()

	log.Printf("Starting server on :%s", port)
	log.Printf("Metrics available at http://localhost:%s/metrics", port)

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
