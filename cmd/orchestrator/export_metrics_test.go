package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportMetrics(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/export/metrics", r.URL.Path)
		assert.Equal(t, "all", r.URL.Query().Get("state"))

		w.Header().Set("Content-Type", "text/csv")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("JobID,Status,cpu_usage\nJOB-1,Running,15.5\n"))
	}))
	defer server.Close()

	// Capture stdout
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Avoid os.Exit
	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	t.Run("ExportToStdout", func(t *testing.T) {
		buf.Reset()
		exportMetrics(server.URL, "-", "all")

		assert.False(t, exitCalled)
		output := buf.String()
		assert.Contains(t, output, "JobID,Status,cpu_usage\nJOB-1,Running,15.5\n")
		assert.NotContains(t, output, "Metrics successfully exported to")
	})

	t.Run("ExportToFile", func(t *testing.T) {
		buf.Reset()

		tempDir := t.TempDir()
		outFile := filepath.Join(tempDir, "metrics.csv")

		exportMetrics(server.URL, outFile, "all")

		assert.False(t, exitCalled)
		output := buf.String()
		assert.Contains(t, output, "Metrics successfully exported to "+outFile)

		// Verify file contents
		content, err := os.ReadFile(outFile)
		require.NoError(t, err)
		assert.Equal(t, "JobID,Status,cpu_usage\nJOB-1,Running,15.5\n", string(content))
	})
}

func TestExportMetrics_Error(t *testing.T) {
	// Setup mock server returning error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal error"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	exportMetrics(server.URL, "-", "all")

	assert.True(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "Failed to export metrics")
	assert.Contains(t, output, "internal error")
}

func TestExportMetrics_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	exportMetrics("http://localhost:0", "-", "all")

	assert.True(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "Failed to connect to orchestrator")
}

func TestExportMetrics_FileCreationError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("metrics"))
	}))
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	exitCalled := false
	oldExit := exitFunc
	exitFunc = func(code int) { exitCalled = true }
	defer func() { exitFunc = oldExit }()

	// Try to write to a directory
	exportMetrics(server.URL, t.TempDir(), "all")

	assert.True(t, exitCalled)
	output := buf.String()
	assert.Contains(t, output, "Failed to create output file")
}
