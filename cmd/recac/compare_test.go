package main

import (
	"context"
	"fmt"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCompareCmd(t *testing.T) {
	// Setup Mock Session Manager
	mockSM := NewMockSessionManager()

	// Create sessions
	sA := &runner.SessionState{
		Name:           "session-A",
		Status:         "completed",
		Goal:           "Fix bug X",
		StartTime:      time.Now().Add(-10 * time.Minute),
		EndTime:        time.Now().Add(-5 * time.Minute),
		AgentStateFile: "state-A.json",
		EndCommitSHA:   "abcdef123456",
		LogFile:        "logs-A.log",
	}
	sB := &runner.SessionState{
		Name:           "session-B",
		Status:         "running",
		Goal:           "Fix bug X",
		StartTime:      time.Now().Add(-5 * time.Minute),
		AgentStateFile: "state-B.json",
		LogFile:        "logs-B.log",
	}

	mockSM.Sessions["session-A"] = sA
	mockSM.Sessions["session-B"] = sB

	// Override sessionManagerFactory
	origSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = origSMFactory }()

	// Override loadAgentState
	origLoadState := loadAgentState
	loadAgentState = func(path string) (*agent.State, error) {
		if path == "state-A.json" {
			return &agent.State{
				Model: "gpt-4",
				TokenUsage: agent.TokenUsage{
					TotalTokens:         1000,
					TotalPromptTokens:   800,
					TotalResponseTokens: 200,
				},
			}, nil
		}
		if path == "state-B.json" {
			return &agent.State{
				Model: "claude-3-opus",
				TokenUsage: agent.TokenUsage{
					TotalTokens:         500,
					TotalPromptTokens:   400,
					TotalResponseTokens: 100,
				},
			}, nil
		}
		return nil, fmt.Errorf("not found")
	}
	defer func() { loadAgentState = origLoadState }()

	// Test case: Basic Comparison
	output, err := executeCommand(rootCmd, "compare", "session-A", "session-B")
	assert.NoError(t, err)
	assert.Contains(t, output, "SESSION A (session-A)")
	assert.Contains(t, output, "SESSION B (session-B)")
	assert.Contains(t, output, "Status")
	assert.Contains(t, output, "gpt-4")
	assert.Contains(t, output, "claude-3-opus")
	assert.Contains(t, output, "1000")    // Tokens A
	assert.Contains(t, output, "500")     // Tokens B
	assert.Contains(t, output, "abcdef1") // Commit SHA prefix

	// Test case: Analysis
	mockAgent := new(MockAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("Agent A was better because...", nil)

	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	outputAnalyze, err := executeCommand(rootCmd, "compare", "session-A", "session-B", "--analyze")
	assert.NoError(t, err)
	assert.Contains(t, outputAnalyze, "AI Analysis")
	assert.Contains(t, outputAnalyze, "Agent A was better because")
	assert.Contains(t, outputAnalyze, "Analyzing logs with AI")

	// Verify Mock Logs were "read" (mock SM returns dummy logs)
	// The prompt should contain the dummy log content
	// We can't easily verify the prompt sent to the mock agent here without more complex setup,
	// but the fact it returned the canned response means it was called.
	mockAgent.AssertExpectations(t)
}

func TestCompareCmd_Errors(t *testing.T) {
	// Setup Mock Session Manager
	mockSM := NewMockSessionManager()
	origSMFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = origSMFactory }()

	// Case: Not enough args
	_, err := executeCommand(rootCmd, "compare", "session-A")
	assert.Error(t, err) // Cobra argument validation error

	// Case: Session not found
	_, err = executeCommand(rootCmd, "compare", "non-existent", "other")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to load session A")
}
