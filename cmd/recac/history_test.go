package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/AlecAivazis/survey/v2"
)

// MockSessionManager is a mock that implements ISessionManager for testing.
type MockSessionManager struct {
	Sessions             []*runner.SessionState
	Err                  error
	IsProcessRunningFunc func(pid int) bool
}

func (m *MockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.Sessions, m.Err
}

func (m *MockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	for _, s := range m.Sessions {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, fmt.Errorf("session '%s' not found", name)
}

func (m *MockSessionManager) IsProcessRunning(pid int) bool {
	if m.IsProcessRunningFunc != nil {
		return m.IsProcessRunningFunc(pid)
	}
	// Default mock behavior
	return pid%2 != 0 // Let's consider odd PIDs as running for this mock
}

// Implement unused methods to satisfy the interface
func (m *MockSessionManager) SaveSession(s *runner.SessionState) error    { return nil }
func (m *MockSessionManager) StopSession(name string) error                 { return nil }
func (m *MockSessionManager) GetSessionLogs(name string) (string, error)    { return "", nil }
func (m *MockSessionManager) StartSession(name string, command []string, workspace string) (*runner.SessionState, error) {
	return nil, nil
}
func (m *MockSessionManager) GetSessionPath(name string) string { return "" }


// runCommand executes a cobra command and captures its output.
func runCommand(cmd *cobra.Command, args ...string) (string, error) {
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs(args)

	err := cmd.Execute()
	return buf.String(), err
}

// setupTestSessions creates a slice of sessions with varying properties for testing.
func setupTestSessions(t *testing.T) []*runner.SessionState {
	t.Helper()

	// createAgentState creates a temporary agent state file and returns its path.
	createAgentState := func(name string, model string, promptTokens, completionTokens int) string {
		tmpDir := t.TempDir()
		state := agent.State{
			Model: model,
			TokenUsage: agent.TokenUsage{
				TotalPromptTokens:     promptTokens,
				TotalResponseTokens: completionTokens,
				TotalTokens:      promptTokens + completionTokens,
			},
		}
		filePath := filepath.Join(tmpDir, name+"_agent_state.json")
		data, err := json.Marshal(state)
		require.NoError(t, err)
		err = os.WriteFile(filePath, data, 0644)
		require.NoError(t, err)
		return filePath
	}

	// createLogFile creates a temporary log file.
	createLogFile := func(name string, content string) string {
		tmpDir := t.TempDir()
		filePath := filepath.Join(tmpDir, name+".log")
		err := os.WriteFile(filePath, []byte(content), 0644)
		require.NoError(t, err)
		return filePath
	}

	return []*runner.SessionState{
		{
			Name:           "session-1-completed",
			Status:         "completed",
			StartTime:      time.Now().Add(-2 * time.Hour),
			EndTime:        time.Now().Add(-1 * time.Hour),
			PID:            101,
			Type:           "detached",
			Workspace:      "/tmp/workspace1",
			LogFile:        createLogFile("session-1", "line1\nline2\n"),
			AgentStateFile: createAgentState("session-1", "gemini-pro", 100, 200),
		},
		{
			Name:           "session-2-failed",
			Status:         "failed",
			StartTime:      time.Now().Add(-4 * time.Hour),
			EndTime:        time.Now().Add(-3 * time.Hour),
			PID:            102,
			Type:           "interactive",
			Workspace:      "/tmp/workspace2",
			LogFile:        createLogFile("session-2", "error log\n"),
			Error:          "Process exited with code 1",
			AgentStateFile: createAgentState("session-2", "gpt-4", 500, 0),
		},
		{
			Name:           "session-3-running",
			Status:         "running",
			StartTime:      time.Now().Add(-10 * time.Minute),
			PID:            103, // Odd PID, so IsProcessRunning will be true
			Workspace:      "/tmp/workspace3",
			LogFile:        createLogFile("session-3", "still running...\n"),
			AgentStateFile: createAgentState("session-3", "ollama", 10, 10),
		},
	}
}


// TestHistoryCmd_DetailView tests the command when a session name is provided directly.
func TestHistoryCmd_DetailView(t *testing.T) {
	// Setup
	sessions := setupTestSessions(t)
	mockSM := &MockSessionManager{
		Sessions: sessions,
		IsProcessRunningFunc: func(pid int) bool { return pid == 103 },
	}

	// Replace the factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Create root command and initialize history command
	rootCmd := &cobra.Command{Use: "recac"}
	initHistoryCmd(rootCmd)

	// Execute
	output, err := runCommand(rootCmd, "history", "session-1-completed")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "Session Details for 'session-1-completed'")
	require.Regexp(t, `Status:\s+completed`, output)
	assert.Contains(t, output, "Duration:")
	require.Regexp(t, `Model:\s+gemini-pro`, output)
	require.Regexp(t, `Total Tokens:\s+300`, output)
	assert.Contains(t, output, "Estimated Cost:")
	assert.Contains(t, output, "Recent Logs")
	assert.Contains(t, output, "line1\nline2")
}


// TestHistoryCmd_NoSessions tests the interactive mode when no sessions exist.
func TestHistoryCmd_NoSessions(t *testing.T) {
	// Setup
	mockSM := &MockSessionManager{Sessions: []*runner.SessionState{}}
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	rootCmd := &cobra.Command{Use: "recac"}
	initHistoryCmd(rootCmd)

	// Execute
	output, err := runCommand(rootCmd, "history")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, output, "No sessions found.")
}

// TestHistoryCmd_InteractiveSelection tests the interactive session selection flow.
func TestHistoryCmd_InteractiveSelection(t *testing.T) {
	// Setup
	sessions := setupTestSessions(t)
	mockSM := &MockSessionManager{
		Sessions: sessions,
		IsProcessRunningFunc: func(pid int) bool { return pid == 103 },
	}

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// This is the magic part for testing survey prompts
	askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		// We expect a Select prompt
		selectPrompt, ok := p.(*survey.Select)
		require.True(t, ok, "Expected a survey.Select prompt")

		// Assert options are correct and in the right order (newest first)
		require.Len(t, selectPrompt.Options, 3)
		assert.Contains(t, selectPrompt.Options[0], "session-3-running")
		assert.Contains(t, selectPrompt.Options[1], "session-1-completed")
		assert.Contains(t, selectPrompt.Options[2], "session-2-failed")

		// Simulate the user selecting the second option ("session-1-completed")
		val := response.(*string)
		*val = selectPrompt.Options[1] // Choose the completed session

		return nil
	}
	// Restore original function after test
	defer func() { askOne = survey.AskOne }()


	rootCmd := &cobra.Command{Use: "recac"}
	initHistoryCmd(rootCmd)

	// Execute
	output, err := runCommand(rootCmd, "history")

	// Assert: Should show details for the selected session
	require.NoError(t, err)
	assert.Contains(t, output, "Session Details for 'session-1-completed'")
	require.Regexp(t, `Status:\s+completed`, output)
	require.Regexp(t, `Model:\s+gemini-pro`, output)
	require.Regexp(t, `Total Tokens:\s+300`, output)
}
