package runner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/git"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushProgress_SavesAgentState(t *testing.T) {
	// Setup Workspace
	tmpDir, err := os.MkdirTemp("", "recac-test-git-ops")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Initialize Git
	gitClient := git.NewClient()
	_, err = gitClient.Run(tmpDir, "init")
	require.NoError(t, err)
	require.NoError(t, gitClient.Config(tmpDir, "user.email", "test@example.com"))
	require.NoError(t, gitClient.Config(tmpDir, "user.name", "Test User"))
	// Make initial commit so we have a HEAD
	_, err = gitClient.Run(tmpDir, "commit", "--allow-empty", "-m", "Initial commit")
	require.NoError(t, err)

	// Checkout feature branch because pushProgress skips main/master
	_, err = gitClient.Run(tmpDir, "checkout", "-b", "feature/test-rollback")
	require.NoError(t, err)

	// Create dummy agent state file
	statePath := filepath.Join(tmpDir, ".agent_state.json")
	expectedState := `{"memory":["foo","bar"]}`
	require.NoError(t, os.WriteFile(statePath, []byte(expectedState), 0644))

	// Create Session
	session := &Session{
		Workspace:     tmpDir,
		Iteration:     1,
		UseLocalAgent: true, // Bypass permissions fix
		Logger:        slog.New(slog.NewTextHandler(io.Discard, nil)),
		SleepFunc:     func(d time.Duration) {},
	}

	// Execution
	session.pushProgress(context.Background())

	// Verification
	// 1. Check if state file was copied to hidden dir
	hiddenStatePath := filepath.Join(tmpDir, ".recac", "state", "agent_state.json")
	require.FileExists(t, hiddenStatePath)

	content, err := os.ReadFile(hiddenStatePath)
	require.NoError(t, err)
	assert.Equal(t, expectedState, string(content))

	// 2. Check if it was committed
	// We check `git ls-tree -r HEAD` to see if .recac/state/agent_state.json is there
	out, err := gitClient.Run(tmpDir, "ls-tree", "-r", "--name-only", "HEAD")
	require.NoError(t, err)
	assert.Contains(t, out, ".recac/state/agent_state.json")

	// 3. Verify commit message
	log, err := gitClient.Log(tmpDir, "-1", "--pretty=%s")
	require.NoError(t, err)
	assert.Equal(t, "chore: progress update (iteration 1)", log[0])
}
