package telemetry

import (
	"fmt"
	"net"
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

func resetMetricsState() {
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()
}

func TestStartMetricsServer(t *testing.T) {
	resetMetricsState()
	defer resetMetricsState()

	port := 19000 // Pick a port unlikely to be used

	// Start server in background
	errChan := make(chan error)
	go func() {
		errChan <- StartMetricsServer(port)
	}()

	// Wait a bit to ensure it starts listening or fails
	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("StartMetricsServer failed: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		// Check if port is listening
		conn, err := net.Dial("tcp", fmt.Sprintf(":%d", port))
		if err != nil {
			t.Fatalf("Failed to connect to metrics server: %v", err)
		}
		conn.Close()
	}

	// Try to start again - should return nil immediately (idempotent)
	if err := StartMetricsServer(port); err != nil {
		t.Errorf("Subsequent StartMetricsServer call failed: %v", err)
	}
}

func TestStartMetricsServer_Conflict(t *testing.T) {
	resetMetricsState()
	defer resetMetricsState()

	basePort := 19100

	// Occupy the first port
	l1, err := net.Listen("tcp", fmt.Sprintf(":%d", basePort))
	if err != nil {
		t.Skipf("Could not bind port %d, skipping conflict test: %v", basePort, err)
	}
	defer l1.Close()

	// Occupy the second port
	l2, err := net.Listen("tcp", fmt.Sprintf(":%d", basePort+1))
	if err != nil {
		t.Skipf("Could not bind port %d, skipping conflict test: %v", basePort+1, err)
	}
	defer l2.Close()

	// Start server in background - should pick basePort+2
	errChan := make(chan error)
	go func() {
		errChan <- StartMetricsServer(basePort)
	}()

	select {
	case err := <-errChan:
		if err != nil {
			t.Fatalf("StartMetricsServer failed: %v", err)
		}
	case <-time.After(100 * time.Millisecond):
		// Check if it skipped occupied ports and bound to basePort+2
		conn, err := net.Dial("tcp", fmt.Sprintf(":%d", basePort+2))
		if err != nil {
			t.Fatalf("Failed to connect to metrics server on fallback port: %v", err)
		}
		conn.Close()
	}
}

func TestStartMetricsServer_AllPortsBusy(t *testing.T) {
	resetMetricsState()
	defer resetMetricsState()

	basePort := 19200
	var listeners []net.Listener

	// Occupy all 10 ports
	for i := 0; i < 10; i++ {
		l, err := net.Listen("tcp", fmt.Sprintf(":%d", basePort+i))
		if err != nil {
			t.Skipf("Could not bind port %d, skipping busy test", basePort+i)
		}
		listeners = append(listeners, l)
	}
	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()

	err := StartMetricsServer(basePort)
	if err == nil {
		t.Error("Expected error when all ports are busy, got nil")
	}
}
