package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
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
