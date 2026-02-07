package main

import (
	"bytes"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestInspectCmd(t *testing.T) {
	// Setup Mock Session Manager
	mockSM := NewMockSessionManager()

	// Create a dummy session
	sessionName := "test-session"
	agentStateFile := "/tmp/test-session-state.json"

	session := &runner.SessionState{
		Name:           sessionName,
		Status:         "running",
		AgentStateFile: agentStateFile,
	}
	mockSM.Sessions[sessionName] = session

	// Mock sessionManagerFactory
	oldFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = oldFactory }()

	// Mock loadAgentState
	oldLoadAgentState := loadAgentState
	loadAgentState = func(filePath string) (*agent.State, error) {
		if filePath != agentStateFile {
			return nil, assert.AnError
		}
		return &agent.State{
			Model: "gpt-4",
			LastActivity: time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC),
			TokenUsage: agent.TokenUsage{
				TotalTokens: 100,
				TotalPromptTokens: 50,
				TotalResponseTokens: 50,
			},
			MaxTokens: 8000,
			Memory: []string{"Remember to write tests."},
			History: []agent.Message{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there"},
			},
		}, nil
	}
	defer func() { loadAgentState = oldLoadAgentState }()

	// Execute command
	cmd := inspectCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Reset flags
	cmd.Flags().Set("json", "false")
	cmd.Flags().Set("history", "5")

	err := cmd.RunE(cmd, []string{sessionName})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Session: test-session")
	assert.Contains(t, output, "Model: gpt-4")
	assert.Contains(t, output, "Tokens: 100 / 8000")
	assert.Contains(t, output, "Remember to write tests")
	assert.Contains(t, output, "[user] Hello")
}

func TestInspectCmd_JSON(t *testing.T) {
	// Setup Mock Session Manager
	mockSM := NewMockSessionManager()

	sessionName := "test-session-json"
	agentStateFile := "/tmp/test-session-json-state.json"

	session := &runner.SessionState{
		Name:           sessionName,
		Status:         "running",
		AgentStateFile: agentStateFile,
	}
	mockSM.Sessions[sessionName] = session

	// Mock factories
	oldFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = oldFactory }()

	oldLoadAgentState := loadAgentState
	loadAgentState = func(filePath string) (*agent.State, error) {
		return &agent.State{
			Model: "gpt-4",
		}, nil
	}
	defer func() { loadAgentState = oldLoadAgentState }()

	// Execute command
	cmd := inspectCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	cmd.Flags().Set("json", "true")

	err := cmd.RunE(cmd, []string{sessionName})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, `"model": "gpt-4"`)
}

func TestInspectCmd_Latest(t *testing.T) {
	// Setup Mock Session Manager
	mockSM := NewMockSessionManager()

	// Create multiple sessions
	s1 := &runner.SessionState{Name: "s1", StartTime: time.Now().Add(-2 * time.Hour), AgentStateFile: "/tmp/s1.json"}
	s2 := &runner.SessionState{Name: "s2", StartTime: time.Now().Add(-1 * time.Hour), AgentStateFile: "/tmp/s2.json"}
	mockSM.Sessions["s1"] = s1
	mockSM.Sessions["s2"] = s2

	// Mock factories
	oldFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = oldFactory }()

	oldLoadAgentState := loadAgentState
	loadAgentState = func(filePath string) (*agent.State, error) {
		return &agent.State{Model: "gpt-4"}, nil
	}
	defer func() { loadAgentState = oldLoadAgentState }()

	// Execute command with NO args
	cmd := inspectCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Reset flags
	cmd.Flags().Set("json", "false")

	err := cmd.RunE(cmd, []string{})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "Inspecting most recent session: s2")
	assert.Contains(t, output, "Session: s2")
}
