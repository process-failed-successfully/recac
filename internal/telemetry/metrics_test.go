package telemetry

import (
	"net"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStartMetricsServer(t *testing.T) {
	// Reset state
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()

	basePort := 9090

	// Start server in background
	go StartMetricsServer(basePort)

	// Wait for server to start
	url := "http://localhost:" + strconv.Itoa(basePort) + "/metrics"
	var resp *http.Response
	var err error

	for i := 0; i < 20; i++ {
		resp, err = http.Get(url)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	assert.NoError(t, err)
	if resp != nil {
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}
}

func TestStartMetricsServer_Conflict(t *testing.T) {
	// Reset state
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()

	basePort := 9092

	// Occupy basePort
	l, err := net.Listen("tcp", ":"+strconv.Itoa(basePort))
	assert.NoError(t, err)
	defer l.Close()

	go StartMetricsServer(basePort)

	// Expect it to listen on basePort + 1
	url := "http://localhost:" + strconv.Itoa(basePort+1) + "/metrics"
	var resp *http.Response

	for i := 0; i < 20; i++ {
		resp, err = http.Get(url)
		if err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}

	assert.NoError(t, err)
	if resp != nil {
		assert.Equal(t, http.StatusOK, resp.StatusCode)
		resp.Body.Close()
	}
}

func TestStartMetricsServer_AlreadyRunning(t *testing.T) {
	metricsMu.Lock()
	metricsRunning = true
	metricsMu.Unlock()

	// Should return nil immediately without blocking/starting
	err := StartMetricsServer(9095)
	assert.NoError(t, err)

	// Reset for other tests potentially (though we manage it explicitly)
	metricsMu.Lock()
	metricsRunning = false
	metricsMu.Unlock()
}

func TestTrackFunctions(t *testing.T) {
	// Call all track functions to ensure coverage and no panics
	project := "test-project"

	TrackLineGenerated(project, 10)
	TrackFileCreated(project)
	TrackFileModified(project)
	TrackBuildResult(project, true)
	TrackBuildResult(project, false)
	TrackAgentIteration(project)
	ObserveAgentLatency(project, 1.5)
	TrackTokenUsage(project, 100)
	TrackAgentStall(project)
	SetContextUsage(project, 50.0)
	SetActiveAgents(project, 2)
	SetTasksPending(project, 5)
	TrackTaskCompleted(project)
	TrackLockContention(project)
	TrackOrchestratorLoop(project)
	TrackError(project, "test_error")
	TrackDBOp(project)
	TrackDockerOp(project)
	TrackDockerError(project)
}
