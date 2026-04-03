package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestExportCosts(t *testing.T) {
	// Mock server that returns a CostStatsResponse
	mockResp := `{
		"total_stats": {
			"total_jobs": 10,
			"total_cost": 5.5,
			"total_tokens_prompt": 10000,
			"total_tokens_completion": 5000
		},
		"tag_stats": [
			{"tag": "backend", "jobs_count": 5, "cost": 3.0}
		],
		"model_stats": [
			{"model": "gpt-4", "jobs_count": 8, "cost": 5.0}
		],
		"top_expensive_jobs": [
			{
				"id": "JOB-1",
				"summary": "Fix bugs",
				"metrics": {"cost_usd": 2.5, "tokens_total": 5000}
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/analyze/costs" {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockResp))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	// Mock stdout and exitFunc
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExitFunc := exitFunc
	exitFunc = func(code int) {
		exitCode = code
		panic("exit")
	}
	defer func() { exitFunc = oldExitFunc }()

	tempDir := t.TempDir()

	t.Run("JSON format", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		jsonPath := filepath.Join(tempDir, "costs.json")

		assert.NotPanics(t, func() {
			exportCosts(server.URL, jsonPath, "json")
		})

		assert.FileExists(t, jsonPath)
		content, err := os.ReadFile(jsonPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), `"total_jobs": 10`)
		assert.Contains(t, string(content), `"tag": "backend"`)
	})

	t.Run("CSV format", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		csvPath := filepath.Join(tempDir, "costs.csv")

		assert.NotPanics(t, func() {
			exportCosts(server.URL, csvPath, "csv")
		})

		assert.FileExists(t, csvPath)
		content, err := os.ReadFile(csvPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "Overall,10,5.5000,10000,5000")
		assert.Contains(t, string(content), "Tag Stats,backend,5,3.0000")
		assert.Contains(t, string(content), "Model Stats,gpt-4,8,5.0000")
		assert.Contains(t, string(content), "Job Details,JOB-1,Fix bugs,2.5000,5000")
	})

	t.Run("Empty TopExpensiveJobs tokens logic", func(t *testing.T) {
		mockResp2 := `{
			"top_expensive_jobs": [
				{
					"id": "JOB-2",
					"summary": "Fix more bugs",
					"metrics": {"tokens_prompt": 1000, "tokens_completion": 500}
				}
			]
		}`
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockResp2))
		}))
		defer server2.Close()

		buf.Reset()
		exitCode = 0
		csvPath := filepath.Join(tempDir, "costs_tokens.csv")

		assert.NotPanics(t, func() {
			exportCosts(server2.URL, csvPath, "csv")
		})

		assert.FileExists(t, csvPath)
		content, err := os.ReadFile(csvPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "Job Details,JOB-2,Fix more bugs,0.0000,1500")
	})

	t.Run("Output to stdout (-)", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		assert.NotPanics(t, func() {
			exportCosts(server.URL, "-", "json")
		})
		assert.Contains(t, buf.String(), `"total_jobs": 10`)
	})

	t.Run("Bad URL Error", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		assert.Panics(t, func() {
			exportCosts("http://[::1]:namedport", "-", "json")
		})
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to parse host URL")
	})

	t.Run("API Connection Error", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		assert.Panics(t, func() {
			exportCosts("http://localhost:12345", "-", "json")
		})
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
	})

	t.Run("Bad JSON API Response Error", func(t *testing.T) {
		mockResp2 := `{ invalid json }`
		server2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(mockResp2))
		}))
		defer server2.Close()

		buf.Reset()
		exitCode = 0
		assert.Panics(t, func() {
			exportCosts(server2.URL, "-", "json")
		})
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})

	t.Run("File Create Error", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		badPath := filepath.Join(tempDir, "nonexistent", "costs.json")

		assert.Panics(t, func() {
			exportCosts(server.URL, badPath, "json")
		})
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create output file")
	})

	t.Run("API Error", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		errPath := filepath.Join(tempDir, "error.json")

		assert.Panics(t, func() {
			exportCosts(server.URL+"/invalid", errPath, "json")
		})
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to fetch cost analysis")
	})
}
