package telemetry

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestMetricsHelpers(t *testing.T) {
	project := "test-project"

	// Call all helper functions to ensure they don't panic and cover lines
	TrackLineGenerated(project, 10)
	TrackFileCreated(project)
	TrackFileModified(project)
	TrackBuildResult(project, true)
	TrackBuildResult(project, false)
	TrackAgentIteration(project)
	ObserveAgentLatency(project, 0.5)
	TrackTokenUsage(project, 100)
	TrackAgentStall(project)
	SetContextUsage(project, 50.0)
	SetActiveAgents(project, 2)
	SetTasksPending(project, 5)
	TrackTaskCompleted(project)
	TrackLockContention(project)
	TrackOrchestratorLoop(project)
	TrackError(project, "db_error")
	TrackDBOp(project)
	TrackDockerOp(project)
	TrackDockerError(project)
}

func TestStartMetricsServer(t *testing.T) {
	// First reset state in case other tests ran and didn't shut down cleanly yet
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()

	// Use port 0 to let the OS choose a free port
	srv, port, err := StartMetricsServer(0)
	if err != nil {
		t.Fatalf("Failed to start metrics server: %v", err)
	}
	defer srv.Shutdown(context.Background())

	if port <= 0 {
		t.Errorf("Expected positive port, got %d", port)
	}

	// Wait for server to start
	time.Sleep(100 * time.Millisecond)

	// Verify health check
	healthURL := fmt.Sprintf("http://localhost:%d/healthz", port)
	resp, err := http.Get(healthURL)
	if err != nil {
		t.Fatalf("Failed to request health check: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Health check returned status %d, expected 200", resp.StatusCode)
	}

	// Verify metrics endpoint
	metricsURL := fmt.Sprintf("http://localhost:%d/metrics", port)
	respMetrics, err := http.Get(metricsURL)
	if err != nil {
		t.Fatalf("Failed to request metrics: %v", err)
	}
	defer respMetrics.Body.Close()

	if respMetrics.StatusCode != http.StatusOK {
		t.Errorf("Metrics endpoint returned status %d, expected 200", respMetrics.StatusCode)
	}
}

func TestStartMetricsServer_AlreadyRunning(t *testing.T) {
	// First reset state in case other tests ran and didn't shut down cleanly yet
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()

	srv, _, err := StartMetricsServer(0)
	if err != nil {
		t.Fatalf("First start failed: %v", err)
	}
	defer srv.Shutdown(context.Background())

	// Try starting again
	_, _, err = StartMetricsServer(0)
	if err == nil {
		t.Error("Expected error when starting already running server")
	}
}
