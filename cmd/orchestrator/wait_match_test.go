package main

import (
	"bytes"
	"net/http"
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestWaitMatchCommand(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "all", r.URL.Query().Get("state"))
		assert.Equal(t, "some-regex", r.URL.Query().Get("match"))

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-1", "status": "Completed"}
		]`))
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	viper.Reset()
	viper.Set("orchestrator.wait_match", "some-regex")
	viper.Set("orchestrator.host", server.URL)

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
	assert.Contains(t, out.String(), "All jobs matching 'some-regex' completed successfully.")
}

func TestWaitForMatch_AllCompleted(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs", r.URL.Path)
		assert.Equal(t, "all", r.URL.Query().Get("state"))
		assert.Equal(t, "some-regex", r.URL.Query().Get("match"))

		callCount++
		w.WriteHeader(http.StatusOK)

		if callCount == 1 {
			// First call: jobs are pending/running
			w.Write([]byte(`[
				{"id": "job-1", "status": "Running"},
				{"id": "job-2", "status": "Pending"}
			]`))
		} else {
			// Second call: all completed
			w.Write([]byte(`[
				{"id": "job-1", "status": "Completed"},
				{"id": "job-2", "status": "Completed"}
			]`))
		}
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForMatch(server.URL, "some-regex", &buf)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "Waiting for jobs matching 'some-regex' to complete...")
	assert.Contains(t, buf.String(), "All jobs matching 'some-regex' completed successfully.")
	assert.Equal(t, 2, callCount)
}

func TestWaitForMatch_OneFailed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-1", "status": "Completed"},
			{"id": "job-2", "status": "Failed", "error": "exit code 1"}
		]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForMatch(server.URL, "some-regex", &buf)

	assert.Error(t, err)
	assert.Equal(t, "job job-2 failed with error: exit code 1", err.Error())
	assert.Contains(t, buf.String(), "Waiting for jobs matching 'some-regex' to complete...")
}

func TestWaitForMatch_OneCanceled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[
			{"id": "job-1", "status": "Completed"},
			{"id": "job-2", "status": "Canceled", "error": "user aborted"}
		]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForMatch(server.URL, "some-regex", &buf)

	assert.Error(t, err)
	assert.Equal(t, "job job-2 canceled with error: user aborted", err.Error())
}

func TestWaitForMatch_NoJobs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`[]`))
	}))
	defer server.Close()

	var buf bytes.Buffer
	err := waitForMatch(server.URL, "some-regex", &buf)

	assert.NoError(t, err)
	assert.Contains(t, buf.String(), "No jobs found matching 'some-regex'.")
}
