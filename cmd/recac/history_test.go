package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// setupHistoryTestEnvironment creates a temporary directory structure for history tests.
func setupHistoryTestEnvironment(t *testing.T) (sessionsDir, agentStateFile string) {
	baseTempDir := t.TempDir()
	sessionsDir = filepath.Join(baseTempDir, "sessions")
	assert.NoError(t, os.Mkdir(sessionsDir, 0755))

	agentStateDir := filepath.Join(baseTempDir, "workspace")
	assert.NoError(t, os.Mkdir(agentStateDir, 0755))
	agentStateFile = filepath.Join(agentStateDir, ".agent_state.json")
	return
}

// createMockSession creates a mock session and agent state for testing.
func createMockSession(t *testing.T, sessionsDir, agentStateFile, sessionName, status, model, errMsg string, promptTokens, completionTokens int) {
	agentState := agent.State{
		Model: model,
		TokenUsage: agent.TokenUsage{
			PromptTokens:     promptTokens,
			CompletionTokens: completionTokens,
			TotalTokens:      promptTokens + completionTokens,
		},
	}
	agentStateData, err := json.Marshal(agentState)
	assert.NoError(t, err)
	err = os.WriteFile(agentStateFile, agentStateData, 0644)
	assert.NoError(t, err)

	session := &runner.SessionState{
		Name:           sessionName,
		Status:         status,
		StartTime:      time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		EndTime:        time.Date(2023, 1, 1, 12, 30, 0, 0, time.UTC),
		AgentStateFile: agentStateFile,
		Error:          errMsg,
	}
	sessionData, _ := json.Marshal(session)
	sessionFilePath := filepath.Join(sessionsDir, sessionName+".json")
	os.WriteFile(sessionFilePath, sessionData, 0644)
}

func TestHistoryCmd_ListView(t *testing.T) {
	sessionsDir, agentStateFile := setupHistoryTestEnvironment(t)
	createMockSession(t, sessionsDir, agentStateFile, "test-session-1", "completed", "gpt-4", "", 100000, 200000)

	var buf bytes.Buffer
	// Mock the factory and args for the list view
	mockFactory := func() (*runner.SessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsDir)
	}

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runHistoryCmd(mockFactory, []string{})
	assert.NoError(t, err)

	// Restore stdout and capture output
	w.Close()
	os.Stdout = oldStdout
	buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "test-session-1")
	assert.Contains(t, output, "completed")
	assert.Contains(t, output, "300000")   // Total Tokens
	assert.Contains(t, output, "15.0000")    // Estimated Cost for gpt-4 (30*0.1 + 60*0.2)
	assert.Contains(t, output, "2023-01-01")
}

func TestHistoryCmd_DetailView_Success(t *testing.T) {
	sessionsDir, agentStateFile := setupHistoryTestEnvironment(t)
	createMockSession(t, sessionsDir, agentStateFile, "detailed-session", "failed", "gemini-pro", "Something went wrong", 50000, 150000)

	var buf bytes.Buffer
	mockFactory := func() (*runner.SessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsDir)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runHistoryCmd(mockFactory, []string{"detailed-session"})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	buf.ReadFrom(r)
	output := buf.String()

	assert.Contains(t, output, "detailed-session")
	assert.Contains(t, output, "failed")
	assert.Contains(t, output, "30m0s") // Duration
	assert.Contains(t, output, "gemini-pro")
	assert.Contains(t, output, "50000")
	assert.Contains(t, output, "150000")
	assert.Contains(t, output, "200000")
	assert.Contains(t, output, "0.3500") // Cost for gemini-pro (1*0.05 + 2*0.15)
	assert.Contains(t, output, "Something went wrong")
}

func TestHistoryCmd_SessionNotFound(t *testing.T) {
	sessionsDir, _ := setupHistoryTestEnvironment(t)
	mockFactory := func() (*runner.SessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsDir)
	}

	err := runHistoryCmd(mockFactory, []string{"non-existent-session"})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, ErrSessionNotFound), "Expected ErrSessionNotFound")
	assert.Contains(t, err.Error(), "non-existent-session")
}

func TestHistoryCmd_NoCompletedSessions(t *testing.T) {
	sessionsDir, _ := setupHistoryTestEnvironment(t)
	var buf bytes.Buffer
	mockFactory := func() (*runner.SessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsDir)
	}

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runHistoryCmd(mockFactory, []string{})
	assert.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout
	buf.ReadFrom(r)

	assert.Equal(t, "No completed sessions found.\n", buf.String())
}

func TestHistoryCmd_TooManyArguments(t *testing.T) {
	sessionsDir, _ := setupHistoryTestEnvironment(t)
	mockFactory := func() (*runner.SessionManager, error) {
		return runner.NewSessionManagerWithDir(sessionsDir)
	}

	err := runHistoryCmd(mockFactory, []string{"session-1", "session-2"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "too many arguments")
}

func TestHistoryCmd_ModelInStateFile(t *testing.T) {
	sessionsDir, agentStateFile := setupHistoryTestEnvironment(t)

	// Create a mock agent
	mockAgent := agent.NewGeminiClient("dummy-key", "gemini-pro", "test-project").WithMockResponder(func(prompt string) (string, error) {
		return "mock response", nil
	})
	sm := agent.NewStateManager(agentStateFile)
	mockAgent.WithStateManager(sm)

	// Run the agent to generate a state file
	_, err := mockAgent.Send(context.Background(), "hello")
	assert.NoError(t, err)

	// Create a mock session that points to the generated state file
	createMockSession(t, sessionsDir, agentStateFile, "model-test-session", "completed", "gemini-pro", "", 10, 20)

	// Read the state file and check the model
	state, err := loadAgentState(agentStateFile)
	assert.NoError(t, err)
	assert.Equal(t, "gemini-pro", state.Model)
}
