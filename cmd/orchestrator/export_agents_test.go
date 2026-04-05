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

func TestExportAgents(t *testing.T) {
	// Mock server that returns an AgentStatsResponse
	mockResp := `{
		"agents": [
			{
				"agent_provider": "openrouter",
				"agent_model": "gpt-4",
				"total_jobs": 10,
				"successful_jobs": 8,
				"failed_jobs": 2,
				"success_rate": 0.8,
				"average_duration": 10000000000,
				"average_cost": 0.5,
				"total_cost": 5.0,
				"total_tokens": 10000
			}
		]
	}`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/jobs/analyze/agents" {
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
		jsonPath := filepath.Join(tempDir, "agents.json")

		assert.NotPanics(t, func() {
			exportAgents(server.URL, jsonPath, "json")
		})

		assert.FileExists(t, jsonPath)
		content, err := os.ReadFile(jsonPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), `"agent_model": "gpt-4"`)
		assert.Contains(t, string(content), `"success_rate": 0.8`)
	})

	t.Run("CSV format", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		csvPath := filepath.Join(tempDir, "agents.csv")

		assert.NotPanics(t, func() {
			exportAgents(server.URL, csvPath, "csv")
		})

		assert.FileExists(t, csvPath)
		content, err := os.ReadFile(csvPath)
		assert.NoError(t, err)
		assert.Contains(t, string(content), "Agent Provider,Agent Model,Total Jobs,Successful Jobs,Failed Jobs,Success Rate,Average Duration,Average Cost,Total Cost,Total Tokens")
		assert.Contains(t, string(content), "openrouter,gpt-4,10,8,2,0.8000,10s,0.5000,5.0000,10000")
	})

	t.Run("Output to stdout (-)", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		assert.NotPanics(t, func() {
			exportAgents(server.URL, "-", "json")
		})
		assert.Contains(t, buf.String(), `"agent_model": "gpt-4"`)
	})

	t.Run("Bad URL Error", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		assert.Panics(t, func() {
			exportAgents("http://[::1]:namedport", "-", "json")
		})
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to parse host URL")
	})

	t.Run("API Connection Error", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		assert.Panics(t, func() {
			exportAgents("http://localhost:12345", "-", "json")
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
			exportAgents(server2.URL, "-", "json")
		})
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to decode response")
	})

	t.Run("File Create Error", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		badPath := filepath.Join(tempDir, "nonexistent", "agents.json")

		assert.Panics(t, func() {
			exportAgents(server.URL, badPath, "json")
		})
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to create output file")
	})

	t.Run("API Error", func(t *testing.T) {
		buf.Reset()
		exitCode = 0
		errPath := filepath.Join(tempDir, "error.json")

		assert.Panics(t, func() {
			exportAgents(server.URL+"/invalid", errPath, "json")
		})
		assert.Equal(t, 1, exitCode)
		assert.Contains(t, buf.String(), "Failed to fetch agents analysis")
	})
}
