package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/stretchr/testify/assert"
)

func TestCompareJobs(t *testing.T) {
	// Setup jobs
	startTime := time.Now().Add(-1 * time.Hour)
	endTime1 := startTime.Add(10 * time.Minute)
	endTime2 := startTime.Add(12 * time.Minute)

	job1 := orchestrator.JobInfo{
		ID:        "job-1",
		Status:    "Completed",
		StartTime: startTime,
		EndTime:   endTime1,
		WorkItem: orchestrator.WorkItem{
			AgentProvider: "openrouter",
			AgentModel:    "gpt-4",
			EnvVars: map[string]string{
				"SHARED_VAR": "value1",
				"JOB1_VAR":   "value_only_in_1",
				"API_KEY":    "secret_value",
			},
		},
		Metrics: map[string]float64{
			"accuracy": 0.95,
			"speed":    100.0,
		},
		Outputs: map[string]string{
			"result": "success",
			"log":    "everything is fine",
		},
	}

	job2 := orchestrator.JobInfo{
		ID:        "job-2",
		Status:    "Completed",
		StartTime: startTime,
		EndTime:   endTime2,
		WorkItem: orchestrator.WorkItem{
			AgentProvider: "openrouter",
			AgentModel:    "claude-3",
			EnvVars: map[string]string{
				"SHARED_VAR": "value2",
				"JOB2_VAR":   "value_only_in_2",
			},
		},
		Metrics: map[string]float64{
			"accuracy": 0.98,
			"new_metric": 50.0,
		},
		Outputs: map[string]string{
			"result": "success",
			"log":    "even better",
			"extra":  "bonus",
		},
	}

	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/jobs/job-1") {
			json.NewEncoder(w).Encode(job1)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/jobs/job-2") {
			json.NewEncoder(w).Encode(job2)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/jobs/job-missing") {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Not found", http.StatusNotFound)
	}))
	defer server.Close()

	// Capture output
	var out bytes.Buffer
	originalStdout := stdout
	stdout = &out
	defer func() { stdout = originalStdout }()

	// Test successful comparison
	compareJobs(server.URL, "job-1,job-2")

	output := out.String()
	assert.Contains(t, output, "Job Comparison: job-1 vs job-2")
	assert.Contains(t, output, "Status")
	assert.Contains(t, output, "Duration")
	assert.Contains(t, output, "Agent Provider")
	assert.Contains(t, output, "Agent Model")
	assert.Contains(t, output, "Env Vars:")
	assert.Contains(t, output, "Metrics:")
	assert.Contains(t, output, "Outputs:")

	// Check delta calculation for duration and metrics
	assert.Contains(t, output, "(+2m0s)") // Duration delta
	assert.Contains(t, output, "(+0.03)") // Accuracy delta

	// Check secret masking
	assert.Contains(t, output, "***")

	// Test missing job
	var exitCode int
	originalExitFunc := exitFunc
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExitFunc }()

	out.Reset()
	compareJobs(server.URL, "job-1,job-missing")
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Failed to fetch job job-missing")

	// Test invalid format
	exitCode = 0
	out.Reset()
	compareJobs(server.URL, "job-1")
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "expects exactly two job IDs")

	exitCode = 0
	out.Reset()
	compareJobs(server.URL, ",")
	assert.Equal(t, 1, exitCode)
	assert.Contains(t, out.String(), "Invalid job IDs provided")
}
