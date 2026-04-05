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
				FailedJobs:     2,
				FlakyJobs:      1,
				TotalRetries:   2,
				SuccessRate:    90.0,
				FailureRate:    20.0,
				FlakinessRate:  10.0,
				TopFlakyJobs: []flakyJobStat{
					{Summary: "Flaky Task", Occurrences: 1, TotalRetries: 2, AvgRetries: 2.0},
				},
				TopFailingJobs: []failedJobStat{
					{Summary: "Failing Task", Occurrences: 2},
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
	stdout = &buf
	defer func() { stdout = os.Stdout }()

	// Mock exitFunc
	var exitCode int
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = os.Exit }()

	t.Run("JSON to Stdout", func(t *testing.T) {
		buf.Reset()
		exportReliability(server.URL, "-", "json")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), `"total_jobs": 10`)
		assert.Contains(t, buf.String(), `"Flaky Task"`)
	})

	t.Run("CSV to Stdout", func(t *testing.T) {
		buf.Reset()
		exportReliability(server.URL, "-", "csv")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Total Jobs,Successful Jobs")
		assert.Contains(t, buf.String(), "10,8,2,1,2,90.00")
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
}
