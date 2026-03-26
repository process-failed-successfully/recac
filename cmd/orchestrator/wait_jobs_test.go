package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/orchestrator"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestWaitJobsCommand(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-1",
			Status: "Completed",
		}
		json.NewEncoder(w).Encode(job)
	})
	mux.HandleFunc("/jobs/JOB-2", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-2",
			Status: "Completed",
		}
		json.NewEncoder(w).Encode(job)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Redirect stdout to capture the output
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	// Configure viper
	viper.Reset()
	viper.Set("orchestrator.wait_jobs", "JOB-1,JOB-2")
	viper.Set("orchestrator.host", server.URL)

	// Temporarily disable other bool flags that could be set by other tests
	viper.Set("orchestrator.scale", -1)
	viper.Set("orchestrator.list_jobs", false)
	viper.Set("orchestrator.status", false)
	viper.Set("orchestrator.cancel_all", false)
	viper.Set("orchestrator.retry_failed", false)
	viper.Set("orchestrator.pause", false)
	viper.Set("orchestrator.resume", false)
	viper.Set("orchestrator.monitor", false)
	defer viper.Reset()

	// Mock exitFunc
	originalExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	// Execute run function which wraps the logic
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := run(context.Background(), logger)

	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	output := out.String()
	assert.Contains(t, output, "Job JOB-1 completed successfully.")
	assert.Contains(t, output, "Job JOB-2 completed successfully.")
	assert.Contains(t, output, "All 2 jobs completed successfully.")
}

func TestWaitJobsCommand_Failed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-1", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-1",
			Status: "Completed",
		}
		json.NewEncoder(w).Encode(job)
	})
	mux.HandleFunc("/jobs/JOB-FAIL", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-FAIL",
			Status: "Failed",
			Error:  "Something went wrong",
		}
		json.NewEncoder(w).Encode(job)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Redirect stdout to capture the output
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	// Configure viper
	viper.Reset()
	viper.Set("orchestrator.wait_jobs", "JOB-1, JOB-FAIL ")
	viper.Set("orchestrator.host", server.URL)

	// Temporarily disable other bool flags that could be set by other tests
	viper.Set("orchestrator.scale", -1)
	defer viper.Reset()

	// Mock exitFunc
	originalExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_ = run(context.Background(), logger)

	assert.Equal(t, 1, exitCode)
	output := out.String()
	assert.Contains(t, output, "Wait for jobs failed: job JOB-FAIL failed with error: Something went wrong")
}

func TestWaitJobsCommand_Empty(t *testing.T) {
	server := httptest.NewServer(http.NewServeMux())
	defer server.Close()

	// Redirect stdout to capture the output
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	// Configure viper
	viper.Reset()
	viper.Set("orchestrator.wait_jobs", " ,  ,, ") // Just empty spaces and commas
	viper.Set("orchestrator.host", server.URL)

	viper.Set("orchestrator.scale", -1)
	defer viper.Reset()

	originalExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	_ = run(context.Background(), logger)

	assert.Equal(t, 1, exitCode)
	output := out.String()
	assert.Contains(t, output, "No valid job IDs provided to --wait-jobs")
}
