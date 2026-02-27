package main

import (
	"context"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// MockSessionAgent implements agent.Agent
type MockSessionAgent struct {
	Response string
}

func (m *MockSessionAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockSessionAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	if onChunk != nil {
		onChunk(m.Response)
	}
	return m.Response, nil
}

func TestRunSession(t *testing.T) {
	// Mock agent factory
	origAgentFactory := agentClientFactory
	defer func() { agentClientFactory = origAgentFactory }()

	mockAgent := &MockSessionAgent{Response: "Hello from Mock Session"}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock StartSession
	origStartSession := startSessionFunc
	defer func() { startSessionFunc = origStartSession }()

	startSessionCalled := false
	startSessionFunc = func(ag agent.Agent, personaID string) error {
		startSessionCalled = true
		assert.Equal(t, mockAgent, ag)
		assert.Equal(t, "default", personaID) // Default value check
		return nil
	}

	// Execute command
	cmd := &cobra.Command{Use: "recac"}
	cmd.AddCommand(sessionCmd)

	// We need to execute `runSession` directly or via command.
	// Via command requires proper args parsing setup.
	// Calling runSession directly is easier.

	err := runSession(sessionCmd, []string{})
	assert.NoError(t, err)
	assert.True(t, startSessionCalled)
}

func TestRunSessionWithPersona(t *testing.T) {
	// Mock agent factory
	origAgentFactory := agentClientFactory
	defer func() { agentClientFactory = origAgentFactory }()

	mockAgent := &MockSessionAgent{Response: "Hello from Mock Session"}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock StartSession
	origStartSession := startSessionFunc
	defer func() { startSessionFunc = origStartSession }()

	startSessionCalled := false
	startSessionFunc = func(ag agent.Agent, personaID string) error {
		startSessionCalled = true
		assert.Equal(t, mockAgent, ag)
		assert.Equal(t, "security", personaID) // Custom value check
		return nil
	}

	// Set flag manually
	sessionPersona = "security"
	defer func() { sessionPersona = "default" }() // Reset

	err := runSession(sessionCmd, []string{})
	assert.NoError(t, err)
	assert.True(t, startSessionCalled)
}
