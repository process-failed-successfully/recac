package telemetry

import (
	"net/http"
	"testing"
	"time"
)

func TestStartMetricsServer(t *testing.T) {
	addr := "localhost:19090" // Use non-standard port to avoid conflicts
	
	// Start server in goroutine
	go func() {
		StartMetricsServer(addr)
	}()

	// Give it a moment to start
	time.Sleep(100 * time.Millisecond)

	// Check endpoint
	resp, err := http.Get("http://" + addr + "/metrics")
	if err != nil {
		t.Fatalf("Failed to query metrics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}
