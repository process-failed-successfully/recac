package telemetry

import (
	"fmt"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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
	// Reset global state
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()

	// Find a free port to start testing
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	basePort := l.Addr().(*net.TCPAddr).Port
	l.Close()

	// Start server in background
	go func() {
		_ = StartMetricsServer(basePort)
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// Verify /metrics endpoint
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", basePort))
	if err != nil {
		t.Fatalf("Failed to request metrics: %v", err)
	}
	defer resp.Body.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestStartMetricsServer_Conflict(t *testing.T) {
	// Reset global state
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()

	// 1. Occupy a port
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	defer l.Close()
	occupiedPort := l.Addr().(*net.TCPAddr).Port

	// 2. Start Metrics Server on occupied port
	// It should try next port (occupiedPort + 1)
	go func() {
		_ = StartMetricsServer(occupiedPort)
	}()

	// Wait for server to start
	time.Sleep(200 * time.Millisecond)

	// 3. Check if it's running on occupiedPort + 1
	// The loop checks basePort, basePort+1...
	// basePort is occupied (by us).
	// So it should pick basePort+1.

	// Ensure basePort+1 is not also occupied by chance (unlikely but possible)
	// We'll just try connecting.

	nextPort := occupiedPort + 1
	url := fmt.Sprintf("http://localhost:%d/metrics", nextPort)

	resp, err := http.Get(url)
	if err != nil {
		// Try +2
		resp, err = http.Get(fmt.Sprintf("http://localhost:%d/metrics", nextPort+1))
	}

	if err != nil {
		t.Fatalf("Metrics server failed to start on fallback port: %v", err)
	}
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
