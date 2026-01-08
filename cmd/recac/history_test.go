package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"
)

// mockSessionManager is a mock implementation of the ISessionManager for testing.
type mockSessionManager struct {
	sessions []*runner.SessionState
	err      error
}

func (m *mockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.sessions, m.err
}

func (m *mockSessionManager) LoadSession(name string) (*runner.SessionState, error) {
	for _, s := range m.sessions {
		if s.Name == name {
			return s, nil
		}
	}
	return nil, os.ErrNotExist
}

func (m *mockSessionManager) IsProcessRunning(pid int) bool {
	// For testing, assume process is not running unless PID is the current process's PID
	return pid == os.Getpid()
}

func (m *mockSessionManager) StartSession(name string, command []string, workspace string) (*runner.SessionState, error) {
	newState := &runner.SessionState{
		Name:      name,
		PID:       12345, // Dummy PID
		StartTime: time.Now(),
		Command:   command,
		Workspace: workspace,
		Status:    "running",
	}
	m.sessions = append(m.sessions, newState)
	return newState, nil
}

func (m *mockSessionManager) StopSession(name string) error {
	for _, s := range m.sessions {
		if s.Name == name {
			s.Status = "stopped"
			return nil
		}
	}
	return os.ErrNotExist
}

func (m *mockSessionManager) GetSessionLogs(name string) (string, error) {
	for _, s := range m.sessions {
		if s.Name == name {
			return "/tmp/mock.log", nil
		}
	}
	return "", os.ErrNotExist
}

// TestHistoryCmd tests the full execution of the history command.
func TestHistoryCmd(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "recac-history-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a sample agent state file
	agentState := agent.State{
		Model: "gpt-4o",
		TokenUsage: agent.TokenUsage{
			PromptTokens:     100000,
			CompletionTokens: 50000,
		},
	}
	agentStateFile := filepath.Join(tmpDir, "agent-state.json")
	data, err := json.Marshal(agentState)
	if err != nil {
		t.Fatalf("Failed to marshal agent state: %v", err)
	}
	if err := os.WriteFile(agentStateFile, data, 0644); err != nil {
		t.Fatalf("Failed to write agent state file: %v", err)
	}

	// Create a mock session manager
	mockSM := &mockSessionManager{
		sessions: []*runner.SessionState{
			{
				Name:           "test-session",
				Status:         "completed",
				StartTime:      time.Now(),
				AgentStateFile: agentStateFile,
			},
		},
	}

	// Override the factory to return the mock
	originalNewSessionManager := newSessionManager
	newSessionManager = func() (runner.ISessionManager, error) {
		return mockSM, nil
	}
	defer func() { newSessionManager = originalNewSessionManager }()

	// Execute the command and capture output
	rootCmd := &cobra.Command{}
	initHistoryCmd(rootCmd)
	output, err := executeCommand(rootCmd, "history")

	if err != nil {
		t.Fatalf("history command returned an error: %v", err)
	}


	// Check the output
	expectedCost := agent.CalculateCost("gpt-4o", 100000, 50000)
	expectedOutput := []string{
		"NAME", "MODEL", "STATUS", "STARTED", "PROMPT TOKENS", "COMPLETION TOKENS", "COST ($)",
		"test-session", "gpt-4o", "completed",
	}
	for _, s := range expectedOutput {
		if !strings.Contains(output, s) {
			t.Errorf("Output is missing expected string '%s'", s)
		}
	}
	if !strings.Contains(output, fmt.Sprintf("%.4f", expectedCost)) {
		t.Errorf("Output has incorrect cost. Got %s, expected %.4f", output, expectedCost)
	}
}
