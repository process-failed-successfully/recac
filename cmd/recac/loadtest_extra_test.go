package main

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Mock for Agent
type MockLoadTestAgent struct {
	Response string
}

func (m *MockLoadTestAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockLoadTestAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestLoadTest_Analyze(t *testing.T) {
	// Setup Server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Setup Mock Agent Factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockAgent := &MockLoadTestAgent{Response: "AI Analysis: Looks good!"}
	agentClientFactory = func(ctx context.Context, provider, model, path, project string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Setup Config
	// We need to set ltAnalyze flag
	ltAnalyze = true
	defer func() { ltAnalyze = false }()

	ltRequests = 1
	ltConcurrency = 1

	// Run
	cmd := &cobra.Command{Use: "loadtest", RunE: runLoadTest}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// We pass url as args[0]
	err := runLoadTest(cmd, []string{server.URL})
	require.NoError(t, err)

	output := buf.String()
	// Should contain analysis
	assert.Contains(t, output, "AI Analysis: Looks good!")
}
