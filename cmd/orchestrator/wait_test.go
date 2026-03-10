package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"recac/internal/orchestrator"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestWaitJobCommand(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-123",
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
	viper.Set("orchestrator.wait_job", "JOB-123")
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
	assert.Contains(t, out.String(), "Job already completed.")
}

func TestWaitForJob(t *testing.T) {
	// Scenario: Job starts Spawning -> logs -> Completed

	states := []string{"Spawning", "Active", "Completed"}
	currentStateIdx := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-123",
			Status: states[currentStateIdx],
		}
		json.NewEncoder(w).Encode(job)
	})

	mux.HandleFunc("/jobs/JOB-123/logs", func(w http.ResponseWriter, r *http.Request) {
		// Only stream logs if active/spawning
		if currentStateIdx >= 2 { // Completed
			http.Error(w, "Already done", http.StatusNotFound)
			return
		}

		// Advance state to Active if Spawning
		if currentStateIdx == 0 {
			currentStateIdx = 1
		}

		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			// If not flusher, just write all at once
			fmt.Fprintln(w, "Log Line 1")
			fmt.Fprintln(w, "Log Line 2")
			currentStateIdx = 2
			return
		}

		fmt.Fprintf(w, "Log Line 1\n")
		flusher.Flush()
		time.Sleep(100 * time.Millisecond)

		fmt.Fprintf(w, "Log Line 2\n")
		flusher.Flush()

		// After logs, job is completed
		currentStateIdx = 2
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	err := waitForJob(server.URL, "JOB-123", &out)

	assert.NoError(t, err)
	output := out.String()
	assert.Contains(t, output, "Waiting for job JOB-123 to start...")
	assert.Contains(t, output, "--- Log Stream Start ---")
	assert.Contains(t, output, "Log Line 1")
	assert.Contains(t, output, "Log Line 2")
	assert.Contains(t, output, "--- Log Stream End ---")
}

func TestWaitForJob_Failed(t *testing.T) {
	mux := http.NewServeMux()
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

	var out bytes.Buffer
	err := waitForJob(server.URL, "JOB-FAIL", &out)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job failed with error: Something went wrong")
}

func TestWaitForJob_Canceled(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-CANCEL", func(w http.ResponseWriter, r *http.Request) {
		job := orchestrator.JobInfo{
			ID:     "JOB-CANCEL",
			Status: "Canceled",
			Error:  "Canceled by user",
		}
		json.NewEncoder(w).Encode(job)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	err := waitForJob(server.URL, "JOB-CANCEL", &out)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job canceled with error: Canceled by user")
}
