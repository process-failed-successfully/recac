package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestExportAnomalies(t *testing.T) {
	// Start a mock server to intercept the API request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/analyze/anomalies", r.URL.Path)
		assert.Equal(t, "0", r.URL.Query().Get("limit"))

		anomalies := []orchestrator.AnomalyReport{
			{
				JobID:       "job-dur-anom",
				Model:       "gpt-4o",
				Duration:    100 * time.Second,
				DurationDev: 3.5,
				Cost:        0.05,
				CostDev:     0.5,
				Status:      "Completed",
			},
		}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(anomalies)
	}))
	defer server.Close()

	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = originalExitFunc }()

	t.Run("Export JSON to stdout", func(t *testing.T) {
		buf.Reset()
		exportAnomalies(server.URL, "-", "json")
		assert.Contains(t, buf.String(), "job-dur-anom")
	})

	t.Run("Export CSV to stdout", func(t *testing.T) {
		buf.Reset()
		exportAnomalies(server.URL, "-", "csv")
		output := buf.String()
		assert.Contains(t, output, "Job ID,Model,Status,Duration,Duration Dev,Cost,Cost Dev")
		assert.Contains(t, output, "job-dur-anom,gpt-4o,Completed,1m40s,3.50,0.0500,0.50")
	})

	t.Run("Export JSON to file", func(t *testing.T) {
		buf.Reset()
		tempDir := t.TempDir()
		outPath := filepath.Join(tempDir, "anomalies.json")
		exportAnomalies(server.URL, outPath, "json")

		assert.Contains(t, buf.String(), "Successfully exported anomalies analysis")
		content, err := os.ReadFile(outPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "job-dur-anom")
	})
}

func TestExportAnomalies_ErrorHandling(t *testing.T) {
	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = originalExitFunc }()

	t.Run("Invalid Host", func(t *testing.T) {
		buf.Reset()
		exportAnomalies("http://invalid-host:12345", "-", "json")
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("HTTP Error Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}))
		defer server.Close()

		buf.Reset()
		exportAnomalies(server.URL, "-", "json")
		assert.Contains(t, buf.String(), "Failed to fetch anomalies analysis")
	})

	t.Run("Decode Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		buf.Reset()
		exportAnomalies(server.URL, "-", "json")
		assert.Contains(t, buf.String(), "Failed to decode response")
	})

	t.Run("URL Parse Error", func(t *testing.T) {
		buf.Reset()
		// Use a control character to force url.Parse error
		exportAnomalies("http://\x00/", "-", "json")
		assert.Contains(t, buf.String(), "Failed to parse host URL")
	})

	t.Run("Export Writer Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]orchestrator.AnomalyReport{})
		}))
		defer server.Close()

		buf.Reset()
		// Try to write to a root path or invalid path
		exportAnomalies(server.URL, "/root/restricted_dir/file.json", "json")
		assert.Contains(t, buf.String(), "Failed to create output file:")
	})

	t.Run("JSON Encode Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]orchestrator.AnomalyReport{{JobID: "job1"}})
		}))
		defer server.Close()

		buf.Reset()
		// Use error writer for stdout
		originalStdout := stdout
		stdout = &errorWriter{}
		defer func() { stdout = originalStdout }()

		exitCalled := false
		originalExitFunc := exitFunc
		exitFunc = func(code int) {
			exitCalled = true
			assert.Equal(t, 1, code)
		}
		defer func() { exitFunc = originalExitFunc }()

		exportAnomalies(server.URL, "-", "json")
		assert.True(t, exitCalled, "Expected exitFunc to be called due to encoding error")
	})

	t.Run("CSV Write Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode([]orchestrator.AnomalyReport{{JobID: "job1"}})
		}))
		defer server.Close()

		buf.Reset()
		// Use error writer for stdout
		originalStdout := stdout
		stdout = &errorWriter{}
		defer func() { stdout = originalStdout }()

		exitCalled := false
		originalExitFunc := exitFunc
		exitFunc = func(code int) {
			exitCalled = true
			assert.Equal(t, 1, code)
		}
		defer func() { exitFunc = originalExitFunc }()

		exportAnomalies(server.URL, "-", "csv")
		assert.True(t, exitCalled, "Expected exitFunc to be called due to CSV write error")
	})
}

type errorWriter struct{}

func (e *errorWriter) Write(p []byte) (n int, err error) {
	return 0, os.ErrPermission
}
