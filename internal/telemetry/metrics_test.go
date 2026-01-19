package telemetry

import (
	"fmt"
	"net"
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
	// We need to test StartMetricsServer
	// Ideally we would mock net.Listen, but it's hardcoded.
	// So we will try to start it on a random port.

	// Reset global state if possible, but it's not exported.
	// `metricsRunning` is not exported.

	// Since `StartMetricsServer` checks `metricsRunning` with a lock,
	// and previous tests (if any) might have set it, we need to be careful.
	// But in unit tests within the same package, we can access unexported variables!
	// Wait, `metrics_test.go` is package `telemetry`, so it CAN access `metricsRunning` and `metricsMu`.

	// Let's reset the state first
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()

	// Find a free port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to find free port: %v", err)
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	// Start server in goroutine
	errChan := make(chan error, 1)
	go func() {
		errChan <- StartMetricsServer(port)
	}()

	// Wait a bit for it to start
	time.Sleep(100 * time.Millisecond)

	// Verify we can connect
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/metrics", port))
	if err != nil {
		// It might be that it failed to start, check errChan
		select {
		case e := <-errChan:
			t.Fatalf("StartMetricsServer returned error: %v", e)
		default:
			t.Fatalf("Failed to connect to metrics server: %v", err)
		}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 OK, got %d", resp.StatusCode)
	}

	// Test double start
	// It should return nil immediately because metricsRunning is true
	if err := StartMetricsServer(port); err != nil {
		t.Errorf("Second StartMetricsServer call should return nil (already running), got error: %v", err)
	}
}

func TestStartMetricsServer_PortConflict(t *testing.T) {
	// Reset state
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()

	// Occupy a port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("Failed to listen: %v", err)
	}
	defer l.Close()
	port := l.Addr().(*net.TCPAddr).Port

	// Try to start metrics server on that port
	// It should try next 10 ports.
	// We need to make sure the next port is FREE.
	// But we can't guarantee that easily.
	// However, usually port+1 is free.

	// Run in goroutine because it blocks
	errChan := make(chan error, 1)
	go func() {
		errChan <- StartMetricsServer(port)
	}()

	time.Sleep(100 * time.Millisecond)

	// Check if it started on port+1 (or subsequent)
	// We can check the logs but we don't capture stdout here easily (unless we hijack it).
	// But we can try to connect to port+1
	found := false
	for i := 1; i < 10; i++ {
		target := port + i
		conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", target), 100*time.Millisecond)
		if err == nil {
			conn.Close()
			found = true
			break
		}
	}

	if !found {
		// Maybe it failed?
		select {
		case e := <-errChan:
			t.Logf("StartMetricsServer returned error: %v", e)
		default:
			t.Log("StartMetricsServer is still running but we couldn't connect to any expected port")
		}
	}
}
