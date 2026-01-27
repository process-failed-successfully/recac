package telemetry

import (
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

func TestStartMetricsServer_Integration(t *testing.T) {
	port := 19990

	// 1. Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		errCh <- StartMetricsServer(port)
	}()

	// 2. Wait for it to start (simple retry loop)
	url := fmt.Sprintf("http://localhost:%d/metrics", port)
	ready := false
	for i := 0; i < 20; i++ {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}

	if !ready {
		// Check if it exited with error
		select {
		case err := <-errCh:
			if err != nil {
				t.Fatalf("StartMetricsServer failed: %v", err)
			}
		default:
			t.Fatal("Metrics server failed to start or serve /metrics within timeout")
		}
	}

	// 3. Test Idempotency (calling again should return nil immediately)
	if err := StartMetricsServer(port); err != nil {
		t.Errorf("Subsequent StartMetricsServer call should return nil, got %v", err)
	}
}

// TestPortConflict is hard because StartMetricsServer keeps running and we can't stop it easily
// (it uses http.Serve which blocks).
// But since we use a global lock `metricsRunning`, we can't really restart it in the same process
// unless we reset the var via unsafe or reflection (not recommended) or add a Stop function.
// For now, the integration test covers the happy path and the "already running" path.
