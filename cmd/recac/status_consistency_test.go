package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStatusCommand_ShowsGitChanges(t *testing.T) {
	mockSM, cleanup := setupStatusTest(t)
	defer cleanup()

	// --- Setup ---
	tempDir := t.TempDir()
	agentStateFile := filepath.Join(tempDir, ".agent_state.json")

	state := &agent.State{
		Model: "test-model",
		TokenUsage: agent.TokenUsage{
			TotalTokens: 100,
		},
	}
	stateBytes, _ := json.Marshal(state)
	os.WriteFile(agentStateFile, stateBytes, 0644)

	// Setup a session that should trigger Git diff output
	mockSM.Sessions["diff-session"] = &runner.SessionState{
		Name:           "diff-session",
		Status:         "running",
		Goal:           "Change README",
		StartTime:      time.Now(),
		AgentStateFile: agentStateFile,
		StartCommitSHA: "abc",
		EndCommitSHA:   "def",
		Workspace:      "/tmp/workspace",
	}

	// We verify that the mock returns the diff we expect
	mockSM.GetSessionGitDiffStatFunc = func(name string) (string, error) {
		return " M README.md\n 1 file changed, 1 insertion(+)", nil
	}

	// --- Execute ---
	rawOutput, err := executeCommand(rootCmd, "status", "diff-session")
	output := stripAnsi(rawOutput) // Using helper from status_test.go

	// --- Assert ---
	assert.NoError(t, err)
	// This assertion expects the output to contain the diff stat, which it currently doesn't.
	assert.Contains(t, output, "M README.md", "Output should contain git diff stat")
	assert.Contains(t, output, "1 file changed", "Output should contain git diff summary")
}
