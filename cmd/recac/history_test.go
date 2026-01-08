package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"
)

// mockSessionManager is a mock implementation of the SessionManager for testing.
type mockSessionManager struct {
	sessions []*runner.SessionState
	err      error
}

func (m *mockSessionManager) ListSessions() ([]*runner.SessionState, error) {
	return m.sessions, m.err
}

func TestRunHistoryCmd(t *testing.T) {
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

	// Redirect stdout to a buffer
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run the command
	err = runHistoryCmd(func() (*runner.SessionManager, error) {
		sm, _ := runner.NewSessionManager()
		sm.SetLister(mockSM)
		return sm, nil
	})

	// Restore stdout
	w.Close()
	os.Stdout = oldStdout

	if err != nil {
		t.Fatalf("runHistoryCmd returned an error: %v", err)
	}

	// Read the output from the buffer
	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

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
