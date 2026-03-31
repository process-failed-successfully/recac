package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"

	"recac/internal/orchestrator"
)

func TestSimulateExecution(t *testing.T) {
	mux := http.NewServeMux()

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		status := orchestrator.Status{
			MaxConcurrentJobs: 2, // Limit concurrency to 2
		}
		json.NewEncoder(w).Encode(status)
	})

	mux.HandleFunc("/jobs/analyze/durations", func(w http.ResponseWriter, r *http.Request) {
		stats := DurationStats{
			MeanDuration: 60000, // Global mean: 60s
			TagStats: []TagStat{
				{Tag: "fast", MeanDuration: 10000}, // 10s
				{Tag: "slow", MeanDuration: 120000}, // 120s
			},
		}
		json.NewEncoder(w).Encode(stats)
	})

	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") == "active" {
			jobs := []orchestrator.JobInfo{
				{
					ID:     "job-A",
					Status: "Running",
					StartTime: time.Now().Add(-5 * time.Second), // Elapsed 5s
					WorkItem: orchestrator.WorkItem{
						ID:       "job-A",
						Tags:     []string{"fast"}, // Total: 10s -> Remaining: 5s
					},
				},
				{
					ID:     "job-B",
					Status: "Pending",
					WorkItem: orchestrator.WorkItem{
						ID:       "job-B",
						DependsOn: []string{"job-A"},
						Tags:     []string{"slow"}, // Total: 120s
					},
				},
				{
					ID:     "job-C",
					Status: "Pending",
					WorkItem: orchestrator.WorkItem{
						ID:       "job-C",
						Tags:     []string{"fast"}, // Total: 10s
					},
				},
			}
			json.NewEncoder(w).Encode(jobs)
		}
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

	mux.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(orchestrator.Status{MaxConcurrentJobs: 2})
	})

	mux.HandleFunc("/jobs/analyze/durations", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(DurationStats{MeanDuration: 60000})
	})

	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") == "active" {
			jobs := []orchestrator.JobInfo{
				{
					ID:     "job-X",
					Status: "Pending",
					WorkItem: orchestrator.WorkItem{
						ID:       "job-X",
						DependsOn: []string{"job-Y"},
					},
				},
				{
					ID:     "job-Y",
					Status: "Pending",
					WorkItem: orchestrator.WorkItem{
						ID:       "job-Y",
						DependsOn: []string{"job-X"},
					},
				},
			}
			json.NewEncoder(w).Encode(jobs)
		}
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
