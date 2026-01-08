package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"
)

// createMockSession creates a mock session file in the specified directory.
func createMockSession(t *testing.T, dir, name, status, agentStateFile string, pid int) {
	t.Helper()
	session := &runner.SessionState{
		Name:           name,
		Status:         status,
		StartTime:      time.Now(),
		AgentStateFile: agentStateFile,
		PID:            pid,
	}
	data, err := json.Marshal(session)
	if err != nil {
		t.Fatalf("Failed to marshal session: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name+".json"), data, 0644); err != nil {
		t.Fatalf("Failed to write session file: %v", err)
	}
}

func TestRunHistoryCmd_NoSessions(t *testing.T) {
	sessionsDir, mockNewSessionManager := setupTestEnvironment(t)
	defer os.RemoveAll(filepath.Dir(filepath.Dir(sessionsDir)))

	// Redirect stdout to a buffer to capture the output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runHistoryListCmd(mockNewSessionManager)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	expected := "No completed sessions found."
	if !strings.Contains(output, expected) {
		t.Errorf("Expected output to contain '%s', but got: %s", expected, output)
	}
}

func TestRunHistoryCmd_WithCompletedSessions(t *testing.T) {
	sessionsDir, mockNewSessionManager := setupTestEnvironment(t)
	defer os.RemoveAll(filepath.Dir(filepath.Dir(sessionsDir)))
	// Create mock agent state
	agentState := agent.State{
		Model: "test-model",
		TokenUsage: agent.TokenUsage{
			PromptTokens:     1000,
			CompletionTokens: 2000,
			TotalTokens:      3000,
		},
	}
	agentStateFile := filepath.Join(sessionsDir, "agent_state.json")
	agentStateData, _ := json.Marshal(agentState)
	os.WriteFile(agentStateFile, agentStateData, 0644)

	createMockSession(t, sessionsDir, "session1", "completed", agentStateFile, 0)
	createMockSession(t, sessionsDir, "session2", "stopped", "", 0)

	// Redirect stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runHistoryListCmd(mockNewSessionManager)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if !strings.Contains(output, "session1") || !strings.Contains(output, "completed") {
		t.Errorf("Output is missing session1 details")
	}
	if !strings.Contains(output, "session2") || !strings.Contains(output, "stopped") {
		t.Errorf("Output is missing session2 details")
	}
	if !strings.Contains(output, "3000") {
		t.Errorf("Output is missing correct token count")
	}
}

func TestRunHistoryCmd_WithRunningSessions(t *testing.T) {
	sessionsDir, mockNewSessionManager := setupTestEnvironment(t)
	defer os.RemoveAll(filepath.Dir(filepath.Dir(sessionsDir)))

	createMockSession(t, sessionsDir, "session1", "running", "", os.Getpid())

	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runHistoryListCmd(mockNewSessionManager)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if strings.Contains(output, "session1") {
		t.Errorf("Running session should not be listed, but was found in output")
	}
	if !strings.Contains(output, "No completed sessions found.") {
		t.Errorf("Expected 'No completed sessions' message, but got: %s", output)
	}
}

func TestRunHistoryDetailCmd_Success(t *testing.T) {
	sessionsDir, mockNewSessionManager := setupTestEnvironment(t)
	defer os.RemoveAll(filepath.Dir(filepath.Dir(sessionsDir)))

	// Create a mock agent state
	mockAgentState := agent.State{
		Model: "test-model",
		TokenUsage: agent.TokenUsage{
			PromptTokens:     100,
			CompletionTokens: 200,
			TotalTokens:      300,
		},
		FinalError: "something went wrong",
	}

	agentStateFile := filepath.Join(sessionsDir, "agent_state.json")
	agentStateData, _ := json.Marshal(mockAgentState)
	os.WriteFile(agentStateFile, agentStateData, 0644)

	createMockSession(t, sessionsDir, "test-session", "completed", agentStateFile, 12345)

	// Redirect stdout to a buffer to capture the output
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runHistoryDetailCmd(mockNewSessionManager, "test-session")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// Verify the output
	expectedSubstrings := []string{
		"Session Details for 'test-session'",
		"Name", "test-session",
		"Status", "completed",
		"PID", "12345",
		"Model", "test-model",
		"Total Tokens", "300",
		"Final Error", "something went wrong",
	}

	for _, s := range expectedSubstrings {
		if !strings.Contains(output, s) {
			t.Errorf("Output is missing expected substring: '%s'. Full output: %s", s, output)
		}
	}
}

func TestRunHistoryDetailCmd_NotFound(t *testing.T) {
	sessionsDir, mockNewSessionManager := setupTestEnvironment(t)
	defer os.RemoveAll(filepath.Dir(filepath.Dir(sessionsDir)))

	err := runHistoryDetailCmd(mockNewSessionManager, "non-existent-session")
	if err == nil {
		t.Fatal("Expected an error for non-existent session, but got nil")
	}

	expectedError := "failed to load session 'non-existent-session'"
	if !strings.Contains(err.Error(), expectedError) {
		t.Errorf("Expected error to contain '%s', but got: %v", expectedError, err)
	}
}
