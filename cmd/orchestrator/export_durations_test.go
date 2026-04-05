package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"recac/internal/orchestrator"
)

func TestExportDurations(t *testing.T) {
	// Mock HTTP server
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/analyze/durations" {
			stats := durationStatsResponse{
				TotalJobs:      2,
				TotalDuration:  30000.0,
				MeanDuration:   15000.0,
				MedianDuration: 15000.0,
				MinDuration:    10000.0,
				MaxDuration:    20000.0,
				TagStats: []struct {
					Tag          string  `json:"tag"`
					Count        int     `json:"count"`
					MeanDuration float64 `json:"mean_duration_ms"`
				}{
					{Tag: "test", Count: 2, MeanDuration: 15000.0},
				},
				TopSlowest: []orchestrator.JobInfo{
					{
						ID:        "JOB-1",
						StartTime: time.Now(),
						EndTime:   time.Now().Add(20 * time.Second),
					},
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
		exportDurations(server.URL, "-", "json")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), `"total_jobs": 2`)
		assert.Contains(t, buf.String(), `"mean_duration_ms": 15000`)
	})

	t.Run("CSV to Stdout", func(t *testing.T) {
		buf.Reset()
		exportDurations(server.URL, "-", "csv")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Total Jobs,Total Duration")
		assert.Contains(t, buf.String(), "2,30000.00,15000.00")
	})

	t.Run("JSON to File", func(t *testing.T) {
		buf.Reset()
		tmpFile, err := os.CreateTemp("", "durations_*.json")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())

		exportDurations(server.URL, tmpFile.Name(), "json")
		assert.Equal(t, 0, exitCode)
		assert.Contains(t, buf.String(), "Successfully exported duration analysis")

		content, err := os.ReadFile(tmpFile.Name())
		require.NoError(t, err)
		assert.Contains(t, string(content), `"total_jobs": 2`)
	})
}
