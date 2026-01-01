package telemetry

import (
	"net/http"
	"testing"
	"time"
)

func TestStartMetricsServer(t *testing.T) {
	port := 19091

	// Start server in goroutine
	go func() {
		StartMetricsServer(port)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check endpoint
	resp, err := http.Get("http://localhost:19091/metrics")
	if err != nil {
		t.Fatalf("Failed to query metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
