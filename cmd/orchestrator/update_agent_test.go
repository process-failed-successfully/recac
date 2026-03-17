package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

func TestUpdateAgentCommand(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-AGENT-UPDATE/agent", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		var req struct {
			AgentProvider string `json:"agent_provider"`
			AgentModel    string `json:"agent_model"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)

		if req.AgentProvider == "openai" && req.AgentModel == "gpt-4o" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	// Redirect stdout to capture the output
	var out bytes.Buffer
	oldStdout := stdout
	stdout = &out
	defer func() {
		stdout = oldStdout
	}()

	// Configure viper
	viper.Reset()
	viper.Set("orchestrator.update_agent_job", "JOB-AGENT-UPDATE")
	viper.Set("orchestrator.agent_provider_val", "openai")
	viper.Set("orchestrator.agent_model_val", "gpt-4o")
	viper.Set("orchestrator.host", server.URL)

	// Temporarily disable other bool flags that could be set by other tests
	viper.Set("orchestrator.scale", -1)
	viper.Set("orchestrator.list_jobs", false)
	viper.Set("orchestrator.status", false)
	viper.Set("orchestrator.cancel_all", false)
	viper.Set("orchestrator.retry_failed", false)
	viper.Set("orchestrator.pause", false)
	viper.Set("orchestrator.resume", false)
	viper.Set("orchestrator.monitor", false)
	viper.Set("orchestrator.wait", false)
	defer viper.Reset()

	// Mock exitFunc
	originalExit := exitFunc
	exitCode := 0
	exitFunc = func(code int) {
		exitCode = code
	}
	defer func() { exitFunc = originalExit }()

	// Execute run function which wraps the logic
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	err := run(context.Background(), logger)

	assert.NoError(t, err)
	assert.Equal(t, 0, exitCode)
	assert.Contains(t, out.String(), "Job JOB-AGENT-UPDATE agent updated to provider=openai model=gpt-4o")
}

func TestUpdateAgent_Success(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123/agent", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		var reqBody struct {
			AgentProvider string `json:"agent_provider"`
			AgentModel    string `json:"agent_model"`
		}
		err := json.NewDecoder(r.Body).Decode(&reqBody)
		assert.NoError(t, err)
		assert.Equal(t, "openai", reqBody.AgentProvider)
		assert.Equal(t, "gpt-4", reqBody.AgentModel)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"agent_provider": "openai", "agent_model": "gpt-4"}`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateAgent(server.URL, "JOB-123", "openai", "gpt-4")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Job JOB-123 agent updated to provider=openai model=gpt-4")
	assert.Equal(t, 0, exitCode)
}

func TestUpdateAgent_Error(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/JOB-123/agent", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`invalid model`))
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	updateAgent(server.URL, "JOB-123", "openai", "gpt-4")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Failed to update agent: invalid model")
	assert.Equal(t, 1, exitCode)
}

func TestUpdateBulkAgent(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/jobs/agent", func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPut, r.Method)

		tag := r.URL.Query().Get("tag")
		match := r.URL.Query().Get("match")

		var req struct {
			AgentProvider string `json:"agent_provider"`
			AgentModel    string `json:"agent_model"`
		}
		err := json.NewDecoder(r.Body).Decode(&req)
		assert.NoError(t, err)

		if req.AgentProvider == "anthropic" && req.AgentModel == "claude-3" && (tag == "backend" || match == "login") {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"updated": 2}`))
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
	})

	server := httptest.NewServer(mux)
	defer server.Close()

	r, w, _ := os.Pipe()
	oldStdout := stdout
	stdout = w
	defer func() { stdout = oldStdout }()

	var exitCode int
	oldExit := exitFunc
	exitFunc = func(code int) { exitCode = code }
	defer func() { exitFunc = oldExit }()

	// Test by tag
	updateBulkAgent(server.URL, "", "backend", "anthropic", "claude-3")

	// Test by match
	updateBulkAgent(server.URL, "login", "", "anthropic", "claude-3")

	w.Close()
	buf := new(strings.Builder)
	_, _ = io.Copy(buf, r)
	out := buf.String()

	assert.Contains(t, out, "Successfully updated agents for 2 pending jobs")
	assert.Equal(t, 0, exitCode)
}
