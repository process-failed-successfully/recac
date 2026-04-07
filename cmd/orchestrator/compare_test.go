package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestCompareJobs(t *testing.T) {
	// Backup original functions and variables
	originalExit := exitFunc
	originalStdout := stdout

	defer func() {
		exitFunc = originalExit
		stdout = originalStdout
	}()

	now := time.Now()

	// Create mock server
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/job1", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:        "job1",
			Summary:   "Fix bug",
			Status:    "Completed",
			StartTime: now.Add(-10 * time.Minute),
			EndTime:   now.Add(-5 * time.Minute),
			WorkItem: orchestrator.WorkItem{
				AgentProvider: "openrouter",
				AgentModel:    "openai/gpt-4",
			},
			Outputs: map[string]string{
				"file": "main.go",
			},
			Metrics: map[string]float64{
				"cost": 0.05,
			},
		}
		json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/jobs/job2", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:        "job2",
			Summary:   "Fix bug and add tests",
			Status:    "Running",
			StartTime: now.Add(-2 * time.Minute),
			WorkItem: orchestrator.WorkItem{
				AgentProvider: "openrouter",
				AgentModel:    "openai/gpt-4o",
			},
			Outputs: map[string]string{
				"file":  "main.go",
				"tests": "main_test.go",
			},
			Metrics: map[string]float64{
				"cost": 0.15,
			},
		}
		json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/jobs/missing", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	t.Run("Success", func(t *testing.T) {
		out := new(bytes.Buffer)
		stdout = out
		exitCalled := false
		exitFunc = func(code int) { exitCalled = true }

		compareJobs(server.URL, "job1,job2")

		assert.False(t, exitCalled)
		outputStr := out.String()

		assert.Contains(t, outputStr, "Comparing: job1 vs job2")
		assert.Contains(t, outputStr, "Fix bug")
		assert.Contains(t, outputStr, "Fix bug and add tests")
		assert.Contains(t, outputStr, "Completed")
		assert.Contains(t, outputStr, "Running")
		assert.Contains(t, outputStr, "openai/gpt-4")
		assert.Contains(t, outputStr, "openai/gpt-4o")
		assert.Contains(t, outputStr, "--- Outputs ---")
		assert.Contains(t, outputStr, "--- Metrics ---")
	})

	t.Run("Invalid Input format", func(t *testing.T) {
		out := new(bytes.Buffer)
		stdout = out
		exitCalled := false
		exitFunc = func(code int) { exitCalled = true }

		compareJobs(server.URL, "job1")

		assert.True(t, exitCalled)
		assert.Contains(t, out.String(), "expects exactly two job IDs separated by a comma")
	})

	t.Run("Missing Job", func(t *testing.T) {
		out := new(bytes.Buffer)
		stdout = out
		exitCalled := false
		exitFunc = func(code int) { exitCalled = true }

		compareJobs(server.URL, "job1,missing")

		assert.True(t, exitCalled)
		assert.Contains(t, out.String(), "Error fetching job missing")
	})

	t.Run("Empty Job", func(t *testing.T) {
		out := new(bytes.Buffer)
		stdout = out
		exitCalled := false
		exitFunc = func(code int) { exitCalled = true }

		compareJobs(server.URL, "job1, ")

		assert.True(t, exitCalled)
		assert.Contains(t, out.String(), "Job IDs cannot be empty")
	})
}
