package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportReliability(t *testing.T) {
	// Mock HTTP server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/analyze/reliability" {
			stats := reliabilityStatsResponse{
				TotalJobs:      10,
				SuccessfulJobs: 8,
				FailedJobs:     1,
				FlakyJobs:      1,
				TotalRetries:   2,
				SuccessRate:    80.0,
				FailureRate:    10.0,
				FlakinessRate:  10.0,
				TopFlakyJobs: []flakyJobStat{
					{Summary: "Flaky Job 1", Occurrences: 1, TotalRetries: 2, AvgRetries: 2.0},
				},
				TopFailingJobs: []failedJobStat{
					{Summary: "Failed Job 1", Occurrences: 1},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(stats)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	server := httptest.NewServer(handler)
	defer server.Close()

	// Capture stdout
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	// Mock exitFunc
	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	t.Run("JSON to Stdout", func(t *testing.T) {
		buf.Reset()
		exportReliability(server.URL, "-", "json")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), `"total_jobs": 10`)
		assert.Contains(t, buf.String(), `"success_rate": 80`)
	})

	t.Run("CSV to Stdout", func(t *testing.T) {
		buf.Reset()
		exportReliability(server.URL, "-", "csv")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Total Jobs,Successful Jobs,Failed Jobs")
		assert.Contains(t, buf.String(), "10,8,1,1,2,80.00,10.00,10.00")
	})

	t.Run("JSON to File", func(t *testing.T) {
		buf.Reset()
		tmpFile, err := os.CreateTemp("", "reliability_*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		exportReliability(server.URL, tmpFile.Name(), "json")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Successfully exported reliability analysis")

		content, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		assert.Contains(t, string(content), `"total_jobs": 10`)
	})

	t.Run("Connection Error", func(t *testing.T) {
		buf.Reset()
		exportReliability("http://invalid-host:12345", "-", "json")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("Error Response", func(t *testing.T) {
		buf.Reset()
		errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer errServer.Close()
		exportReliability(errServer.URL, "-", "json")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to fetch reliability analysis")
	})

	t.Run("Invalid URL", func(t *testing.T) {
		buf.Reset()
		exportReliability("http://::1", "-", "json")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("Decode Error", func(t *testing.T) {
		buf.Reset()
		errServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`invalid json`))
		}))
		defer errServer.Close()
		exportReliability(errServer.URL, "-", "json")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})

	t.Run("File Create Error", func(t *testing.T) {
		buf.Reset()
		exportReliability(server.URL, "/root/invalid/path/file.json", "json")
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create output file")
	})
}
