package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"recac/internal/orchestrator"
)

func TestAnalyzeAnomalies(t *testing.T) {
	// Start a mock server to intercept the API request
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/jobs/analyze/anomalies", r.URL.Path)

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
			{
				JobID:       "job-cost-anom",
				Model:       "gpt-4o",
				Duration:    10 * time.Second,
				DurationDev: 0.1,
				Cost:        1.50,
				CostDev:     2.5,
				Status:      "Failed",
			},
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(anomalies)
	}))
	defer server.Close()

	// Redirect stdout to capture output
	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	// Prevent exit on error
	originalExitFunc := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = originalExitFunc }()

	analyzeAnomalies(server.URL, 10, "text")

	// Verify the output
	output := buf.String()
	assert.Contains(t, output, "Anomaly Analysis (2 top anomalies)")
	assert.Contains(t, output, "job-dur-anom")
	assert.Contains(t, output, "job-cost-anom")
	assert.Contains(t, output, "3.50σ")
	assert.Contains(t, output, "2.50σ")

	// Test JSON format
	buf.Reset()
	analyzeAnomalies(server.URL, 10, "json")
	var parsed []orchestrator.AnomalyReport
	err := json.Unmarshal(buf.Bytes(), &parsed)
	assert.NoError(t, err)
	assert.Len(t, parsed, 2)
	assert.Equal(t, "job-dur-anom", parsed[0].JobID) // should be sorted by dur dev
}

func TestAnalyzeAnomalies_ErrorHandling(t *testing.T) {
	var buf bytes.Buffer
	originalStdout := stdout
	stdout = &buf
	defer func() { stdout = originalStdout }()

	originalExitFunc := exitFunc
	exitFunc = func(code int) {}
	defer func() { exitFunc = originalExitFunc }()

	t.Run("Connection Error", func(t *testing.T) {
		buf.Reset()
		analyzeAnomalies("http://invalid-host:12345", 10, "text")
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("HTTP Error Response", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}))
		defer server.Close()

		buf.Reset()
		analyzeAnomalies(server.URL, 10, "text")
		assert.Contains(t, buf.String(), "Failed to fetch anomalies analysis: internal server error")
	})

	t.Run("Decode Error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("invalid json"))
		}))
		defer server.Close()

		buf.Reset()
		analyzeAnomalies(server.URL, 10, "text")
		assert.Contains(t, buf.String(), "Failed to decode response")
	})
}
