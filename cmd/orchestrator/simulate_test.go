package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"recac/internal/orchestrator"
)

func TestSimulateExecution(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/simulate", func(w http.ResponseWriter, r *http.Request) {
		report := orchestrator.SimulationReport{
			EstimatedTotalTimeMs: 125000,
			JobsProcessed:        3,
			TotalJobs:            3,
			FinalBottleneckJob:   "job-B",
			Deadlocks:            0,
		}
		json.NewEncoder(w).Encode(report)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	viper.Set("orchestrator.simulate", true)
	viper.Set("orchestrator.host", server.URL)

	// Capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Prevent exit
	var exitCode int
	oldExitFunc := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = oldExitFunc }()

	simulateExecution(server.URL)

	assert.Equal(t, 0, exitCode)

	output := buf.String()

	// We expect:
	// A is running, remaining = 5s. (Uses 1 worker).
	// C is pending, inDegree = 0.
	// C gets assigned immediately, remaining = 10s. (Uses 1 worker, 0 left).
	// B is pending, inDegree = 1 (A).
	//
	// T=0: A (rem=5), C (rem=10)
	// T=5: A finishes. Worker frees up. B's inDegree=0.
	// B starts. remaining = 120s.
	// T=10: C finishes. Worker frees up.
	// T=125 (5 + 120): B finishes.
	//
	// Total estimated time: 125s -> 2m 5s
	// Processed: 3 / 3

	assert.Contains(t, output, "Simulation Report")
	assert.Contains(t, output, "2m5s")
	assert.Contains(t, output, "3 / 3")
	assert.Contains(t, output, "job-B") // Bottleneck
}

func TestSimulateExecution_Deadlock(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/simulate", func(w http.ResponseWriter, r *http.Request) {
		report := orchestrator.SimulationReport{
			EstimatedTotalTimeMs: 0,
			JobsProcessed:        0,
			TotalJobs:            2,
			Deadlocks:            2,
		}
		json.NewEncoder(w).Encode(report)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	simulateExecution(server.URL)

	output := buf.String()
	assert.Contains(t, output, "WARNING: 2 jobs could not be processed")
	assert.Contains(t, output, "0 / 2")
}
