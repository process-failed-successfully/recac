package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSimulateExecution_TableDriven(t *testing.T) {
	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	exitFunc = func(int) {}

	origStdout := stdout
	defer func() { stdout = origStdout }()

	tests := []struct {
		name           string
		responseJSON   string
		responseStatus int
		expectContains []string
	}{
		{
			name:           "Success without deadlocks",
			responseJSON:   `{"total_jobs": 5, "jobs_processed": 5, "estimated_total_time_ms": 10000, "deadlocks": 0}`,
			responseStatus: http.StatusOK,
			expectContains: []string{"Simulation Report", "Jobs Processed:", "Estimated Total Time"},
		},
		{
			name:           "Success with deadlocks",
			responseJSON:   `{"total_jobs": 5, "jobs_processed": 3, "estimated_total_time_ms": 5000, "deadlocks": 2}`,
			responseStatus: http.StatusOK,
			expectContains: []string{"WARNING:", "2 jobs could not be processed"},
		},
		{
			name:           "No jobs to simulate",
			responseJSON:   `{"total_jobs": 0, "jobs_processed": 0, "estimated_total_time_ms": 0, "deadlocks": 0}`,
			responseStatus: http.StatusOK,
			expectContains: []string{"No jobs to simulate"},
		},
		{
			name:           "Server error",
			responseJSON:   `Internal Server Error`,
			responseStatus: http.StatusInternalServerError,
			expectContains: []string{"Failed to fetch simulation report"},
		},
		{
			name:           "Bad JSON",
			responseJSON:   `{bad json}`,
			responseStatus: http.StatusOK,
			expectContains: []string{"Failed to decode response:"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			stdout = &buf

			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/simulate" {
					w.WriteHeader(tt.responseStatus)
					w.Write([]byte(tt.responseJSON))
				} else {
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			defer server.Close()

			simulateExecution(server.URL)

			for _, exp := range tt.expectContains {
				assert.Contains(t, buf.String(), exp)
			}
		})
	}
}

func TestSimulatePipelineFileCmd(t *testing.T) {
	// Create a dummy pipeline file
	tmpDir := t.TempDir()
	pipelineFile := filepath.Join(tmpDir, "pipeline.yaml")
	yamlData := []byte(`
name: CLI Test Pipeline
jobs:
  build:
    summary: Build app
`)
	err := os.WriteFile(pipelineFile, yamlData, 0644)
	require.NoError(t, err)

	// Mock server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/simulate/pipeline?target=build", r.URL.String())
		assert.Equal(t, "application/yaml", r.Header.Get("Content-Type"))

		report := orchestrator.SimulationReport{
			EstimatedTotalTimeMs: 150000, // 150s
			JobsProcessed:        1,
			TotalJobs:            1,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(report)
	})

	server := httptest.NewServer(handler)
	defer server.Close()

	// Capture stdout
	var buf bytes.Buffer
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	// Mock exitFunc
	exitCalled := false
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = os.Exit }()

	simulatePipelineFileCmd(server.URL, pipelineFile, "build")

	assert.False(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "Simulation Report")
	assert.Contains(t, output, "Estimated Total Time:")
	assert.Contains(t, output, "2m30s")
	assert.Contains(t, output, "Jobs Processed:")
	assert.Contains(t, output, "1 / 1")
}

func TestSimulatePipelineFileCmd_Errors(t *testing.T) {
	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	var exitCode int
	exitFunc = func(code int) { exitCode = code }

	origStdout := stdout
	defer func() { stdout = origStdout }()

	t.Run("FileReadError", func(t *testing.T) {
		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		simulatePipelineFileCmd("http://localhost:8080", "nonexistent.yaml", "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to read pipeline file")
	})

	tmpDir := t.TempDir()
	pipelineFile := filepath.Join(tmpDir, "pipeline.yaml")
	os.WriteFile(pipelineFile, []byte(`name: Test`), 0644)

	t.Run("InvalidURL", func(t *testing.T) {
		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		simulatePipelineFileCmd("http://\x00invalid", pipelineFile, "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create request")
	})

	t.Run("ConnectionError", func(t *testing.T) {
		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		simulatePipelineFileCmd("http://localhost:12345", pipelineFile, "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("ServerError", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(`Server Error`))
		}))
		defer server.Close()

		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		simulatePipelineFileCmd(server.URL, pipelineFile, "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to fetch pipeline simulation report: Server Error")
	})

	t.Run("BadJSONResponse", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{bad json}`))
		}))
		defer server.Close()

		var buf bytes.Buffer
		stdout = &buf
		exitCode = 0

		simulatePipelineFileCmd(server.URL, pipelineFile, "")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})
}

func TestSimulateExecution_ConnectionError(t *testing.T) {
	origExit := exitFunc
	defer func() { exitFunc = origExit }()
	var exitCode int
	exitFunc = func(code int) { exitCode = code }

	origStdout := stdout
	defer func() { stdout = origStdout }()

	var buf bytes.Buffer
	stdout = &buf

	simulateExecution("http://localhost:12345")
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
}
