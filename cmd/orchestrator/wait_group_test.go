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
	"time"

	"recac/internal/orchestrator"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestWaitGroupCommand(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		group := r.URL.Query().Get("group")
		if group != "my-group" {
			http.Error(w, "Not found", http.StatusNotFound)
			return
		}

		jobs := []orchestrator.JobInfo{
			{ID: "JOB-1", Status: "Completed", WorkItem: orchestrator.WorkItem{ConcurrencyGroup: "my-group"}},
			{ID: "JOB-2", Status: "Completed", WorkItem: orchestrator.WorkItem{ConcurrencyGroup: "my-group"}},
		}
		json.NewEncoder(w).Encode(jobs)
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
	viper.Set("orchestrator.wait_group", "my-group")
	viper.Set("orchestrator.host", server.URL)

	// Temporarily disable other bool flags
	viper.Set("orchestrator.scale", -1)
	viper.Set("orchestrator.list_jobs", false)
	viper.Set("orchestrator.status", false)
	viper.Set("orchestrator.cancel_all", false)
	viper.Set("orchestrator.retry_failed", false)
	viper.Set("orchestrator.pause", false)
	viper.Set("orchestrator.resume", false)
	viper.Set("orchestrator.monitor", false)
	defer viper.Reset()

	originalExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := run(context.Background(), logger)

	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	output := out.String()
	assert.Contains(t, output, "Waiting for jobs in concurrency group 'my-group' to complete...")
	assert.Contains(t, output, "All jobs in concurrency group 'my-group' completed successfully.")
}

func TestWaitForGroup(t *testing.T) {
	// Scenario: Jobs start as Active, then transition to Completed
	states := [][]orchestrator.JobInfo{
		{
			{ID: "JOB-1", Status: "Active"},
			{ID: "JOB-2", Status: "Active"},
		},
		{
			{ID: "JOB-1", Status: "Completed"},
			{ID: "JOB-2", Status: "Active"},
		},
		{
			{ID: "JOB-1", Status: "Completed"},
			{ID: "JOB-2", Status: "Completed"},
		},
	}
	currentStateIdx := 0

	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		jobs := states[currentStateIdx]
		json.NewEncoder(w).Encode(jobs)

		if currentStateIdx < len(states)-1 {
			currentStateIdx++
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer

	// Run in a goroutine with timeout
	errCh := make(chan error, 1)
	go func() {
		errCh <- waitForGroup(server.URL, "test-group", &out)
	}()

	select {
	case err := <-errCh:
		assert.NoError(t, err)
		output := out.String()
		assert.Contains(t, output, "Waiting for jobs in concurrency group 'test-group' to complete...")
		assert.Contains(t, output, "All jobs in concurrency group 'test-group' completed successfully.")
	case <-time.After(5 * time.Second):
		t.Fatal("waitForGroup timed out")
	}
}

func TestWaitForGroup_Failed(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		jobs := []orchestrator.JobInfo{
			{ID: "JOB-FAIL", Status: "Failed", Error: "Job failed processing"},
		}
		json.NewEncoder(w).Encode(jobs)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	err := waitForGroup(server.URL, "fail-group", &out)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job JOB-FAIL failed with error: Job failed processing")
}

func TestWaitForGroup_Canceled(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		jobs := []orchestrator.JobInfo{
			{ID: "JOB-CANCEL", Status: "Canceled", Error: "User triggered cancellation"},
		}
		json.NewEncoder(w).Encode(jobs)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	err := waitForGroup(server.URL, "cancel-group", &out)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "job JOB-CANCEL canceled with error: User triggered cancellation")
}

func TestWaitForGroup_NoJobs(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		jobs := []orchestrator.JobInfo{}
		json.NewEncoder(w).Encode(jobs)
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	err := waitForGroup(server.URL, "empty-group", &out)

	assert.NoError(t, err)
	assert.Contains(t, out.String(), "No jobs found in concurrency group 'empty-group'.")
}
