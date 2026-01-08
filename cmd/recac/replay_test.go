package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"testing"
)

func TestReplayCmd_Success(t *testing.T) {
	// Isolate HOME for this test
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("RECAC_TEST", "1")

	sessionsDir := filepath.Join(homeDir, ".recac", "sessions")
	os.MkdirAll(sessionsDir, 0755)

	// Create a mock session with a valid agent state file
	agentState := agent.State{
		Model: "test-model",
		TokenUsage: agent.TokenUsage{
			PromptTokens:     10,
			CompletionTokens: 20,
			TotalTokens:      30,
		},
	}
	agentStateFile := filepath.Join(sessionsDir, "agent_state_for_replay.json")
	agentData, _ := json.Marshal(agentState)
	os.WriteFile(agentStateFile, agentData, 0644)

	session := runner.SessionState{
		Name:           "replay-test",
		AgentStateFile: agentStateFile,
		Command:        []string{"/bin/echo", "hello"},
	}
	sessionFile := filepath.Join(sessionsDir, "replay-test.json")
	sessionData, _ := json.Marshal(session)
	os.WriteFile(sessionFile, sessionData, 0644)

	output, err := executeCommand(rootCmd, "replay", "replay-test")
	if err != nil {
		t.Fatalf("replay command failed: %v", err)
	}

	t.Logf("Replay command output: %s", output)

	// Check that the replay was initiated
	if !strings.Contains(output, "Successfully started replay session") {
		t.Error("Expected output to contain 'Successfully started replay session'")
	}

	// Verify that the new session file was created with updated info
	replayedSessionFile := filepath.Join(sessionsDir, "replay-test-replayed.json")
	if _, err := os.Stat(replayedSessionFile); os.IsNotExist(err) {
		t.Fatal("Replayed session file was not created")
	}

	data, _ := os.ReadFile(replayedSessionFile)
	var replayedSession runner.SessionState
	json.Unmarshal(data, &replayedSession)

	if !strings.HasSuffix(replayedSession.Name, "-replayed") {
		t.Errorf("Replayed session name is incorrect: %s", replayedSession.Name)
	}
	if replayedSession.Status != "running" {
		t.Errorf("Replayed session status should be 'running', but got '%s'", replayedSession.Status)
	}
}

func TestReplayCmd_NotFound(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	_, err := executeCommand(rootCmd, "replay", "non-existent-session")
	if err == nil {
		t.Fatal("Expected an error for non-existent session, but got nil")
	}

	if !strings.Contains(err.Error(), "failed to load session") {
		t.Errorf("Expected error message not found in output: %s", err.Error())
	}
}

func TestReplayCmd_AlreadyRunning(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)

	sessionsDir := filepath.Join(homeDir, ".recac", "sessions")
	os.MkdirAll(sessionsDir, 0755)

	session := runner.SessionState{
		Name:    "running-session",
		Status:  "running",
		PID:     os.Getpid(),
		Command: []string{"/bin/echo", "hello"},
	}
	sessionFile := filepath.Join(sessionsDir, "running-session.json")
	sessionData, _ := json.Marshal(session)
	os.WriteFile(sessionFile, sessionData, 0644)

	_, err := executeCommand(rootCmd, "replay", "running-session")
	if err == nil {
		t.Fatal("Expected an error for replaying a running session, but got nil")
	}

	if !strings.Contains(err.Error(), "cannot replay a running session") {
		t.Errorf("Expected error message not found in output: %s", err.Error())
	}
}
