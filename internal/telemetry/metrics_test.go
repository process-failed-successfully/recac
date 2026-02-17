package telemetry

import (
	"testing"
	"time"
	"net"
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
	// Find a free port to ensure we can start
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Skip("No ports available")
	}
	port := l.Addr().(*net.TCPAddr).Port
	l.Close()

	// Start in background
	go func() {
		_ = StartMetricsServer(port)
	}()

	// Give it a moment to start and set the flag
	time.Sleep(200 * time.Millisecond)

	// Test "Already Running" path
	// This call should return nil immediately because metricsRunning is true
	err = StartMetricsServer(port)
	assert.NoError(t, err)
}
