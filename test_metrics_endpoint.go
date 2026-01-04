package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"workspace/internal/metrics"
)

func main() {
	// Initialize metrics
	m := metrics.NewMetrics()

	// Create a test handler
	handler := m.Middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}))

	// Create test server
	server := httptest.NewServer(handler)
	defer server.Close()

	// Make a test request
	resp, err := http.Get(server.URL + "/test")
	if err != nil {
		fmt.Printf("Error making request: %v\n", err)
		return
	}
	fmt.Printf("Test request status: %d\n", resp.StatusCode)

	// Test custom business metrics
	m.JobsCompleted.WithLabelValues("build", "success").Inc()
	m.JobsFailed.WithLabelValues("build", "timeout").Inc()
	m.AgentStatus.WithLabelValues("agent-1", "builder").Set(1)
	m.TasksProcessed.Inc()
	m.TasksInProgress.Inc()
	start := time.Now()
	time.Sleep(10 * time.Millisecond)
	m.TaskProcessingTime.WithLabelValues("build").Observe(time.Since(start).Seconds())

	// Create metrics endpoint
	metricsHandler := promhttp.Handler()
	metricsServer := httptest.NewServer(metricsHandler)
	defer metricsServer.Close()

	// Fetch metrics
	metricsResp, err := http.Get(metricsServer.URL)
	if err != nil {
		fmt.Printf("Error fetching metrics: %v\n", err)
		return
	}
	defer metricsResp.Body.Close()

	// Check response
	if metricsResp.StatusCode != http.StatusOK {
		fmt.Printf("Metrics endpoint returned status: %d\n", metricsResp.StatusCode)
		return
	}

	fmt.Println("Metrics endpoint is working correctly!")
	fmt.Println("Content-Type:", metricsResp.Header.Get("Content-Type"))
}
