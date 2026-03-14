package main

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRenameJob(t *testing.T) {
	// Start a local HTTP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/jobs/old-job-123/rename", r.URL.Path)
		require.Equal(t, "PUT", r.Method)
		require.Equal(t, "application/json", r.Header.Get("Content-Type"))

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "Job renamed")
	}))
	defer server.Close()

	// Redirect stdout to capture output
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Call function
	renameJob(server.URL, "old-job-123", "new-job-123")

	// Validate output
	require.Contains(t, buf.String(), "Job old-job-123 renamed successfully to new-job-123")
}

func TestRenameJob_Error(t *testing.T) {
	// Start a local HTTP server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintln(w, "job old-job-123 not found in pending queue")
	}))
	defer server.Close()

	// Redirect stdout and override exitFunc
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExitFunc := exitFunc
	exitFunc = func(code int) {
		exitCalled = true
	}
	defer func() { exitFunc = oldExitFunc }()

	// Call function
	renameJob(server.URL, "old-job-123", "new-job-123")

	// Validate output
	require.True(t, exitCalled)
	require.Contains(t, buf.String(), "Failed to rename job: job old-job-123 not found in pending queue")
}
