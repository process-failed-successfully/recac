package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestAnalyzeAgents_SuccessJSON(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/agents", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "10", r.URL.Query().Get("limit"))
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"agents": [
				{
					"agent_provider": "openrouter",
					"agent_model": "gpt-4",
					"total_jobs": 10,
					"successful_jobs": 9,
					"success_rate": 0.9,
					"average_duration": 60000000000,
					"average_cost": 0.50,
					"total_cost": 5.0
				}
			]
		}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeAgents(server.URL, 10, "json")

	assert.Equal(t, 0, exitCode)
	out := buf.String()
	assert.Contains(t, out, `"agent_model": "gpt-4"`)
	assert.Contains(t, out, `"average_cost": 0.5`)
}

func TestAnalyzeAgents_SuccessText(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/agents", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{
			"agents": [
				{
					"agent_provider": "openrouter",
					"agent_model": "gpt-4",
					"total_jobs": 10,
					"successful_jobs": 9,
					"success_rate": 0.9,
					"average_duration": 60000000000,
					"average_cost": 0.50,
					"total_cost": 5.0
				}
			]
		}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeAgents(server.URL, 10, "text")

	assert.Equal(t, 0, exitCode)
	out := buf.String()
	assert.Contains(t, out, "AI Agent Performance Analysis")
	assert.Contains(t, out, "gpt-4")
	assert.Contains(t, out, "1m0s")
	assert.Contains(t, out, "90.0%")
}

func TestAnalyzeAgents_ConnectionError(t *testing.T) {
	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeAgents("http://invalid-host:12345", 10, "text")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to connect to orchestrator")
}

func TestAnalyzeAgents_ErrorResponse(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/agents", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeAgents(server.URL, 10, "text")

	assert.Equal(t, 1, exitCode)
	assert.Contains(t, buf.String(), "Failed to fetch agents analysis")
}

func TestAnalyzeAgents_Empty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/analyze/agents", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"agents": []}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	var buf bytes.Buffer
	oldStdout := stdout
	stdout = &buf
	defer func() { stdout = oldStdout }()

	oldExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	analyzeAgents(server.URL, 10, "text")

	assert.Equal(t, 0, exitCode)
	assert.Contains(t, buf.String(), "No valid completed jobs with agent data found.")
}
