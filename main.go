package main

import (
	"net/http"
	"log"
	"os"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"workspace/internal/metrics"
)

func main() {
	// Initialize metrics
	metrics := metrics.NewMetrics()

	// Set up Prometheus metrics endpoint
	http.Handle("/metrics", promhttp.Handler())

	// Get port from environment or use default
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Start server
	log.Printf("Starting server on :%s", port)
	log.Printf("Metrics endpoint available at http://localhost:%s/metrics", port)
	log.Fatal(http.ListenAndServe(":"+port, nil))
}
