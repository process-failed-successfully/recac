package main

import (
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/db"
	"recac/internal/git"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRewindCmd(t *testing.T) {
	// Setup Temp Dir
	tempDir := t.TempDir()
	workspace := filepath.Join(tempDir, "workspace")
	sessionsDir := filepath.Join(tempDir, "sessions")
	os.MkdirAll(workspace, 0755)
	os.MkdirAll(sessionsDir, 0755)

	// Setup Git Repo
	gitClient := git.NewClient()
	_, err := gitClient.Run(workspace, "init")
	require.NoError(t, err)
	gitClient.Run(workspace, "config", "user.email", "test@example.com")
	gitClient.Run(workspace, "config", "user.name", "Test User")

	// Setup DB
	dbPath := filepath.Join(workspace, ".recac.db")
	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	require.NoError(t, err)

	// Commit 1: Iteration 1
	os.WriteFile(filepath.Join(workspace, "file1.txt"), []byte("v1"), 0644)
	gitClient.Run(workspace, "add", ".")
	gitClient.Run(workspace, "commit", "-m", "chore: progress update (iteration 1)")

	// Add Obs 1 (Should persist)
	err = store.SaveObservation("test-session", "Agent", "Obs 1")
	require.NoError(t, err)

	// Commit 2: Iteration 2 (Checkpoint)
	time.Sleep(1 * time.Second) // Ensure timestamp diff
	os.WriteFile(filepath.Join(workspace, "file1.txt"), []byte("v2"), 0644)
	gitClient.Run(workspace, "add", ".")
	gitClient.Run(workspace, "commit", "-m", "chore: progress update (iteration 2)")

	// Add Obs 3 (Should be deleted)
	time.Sleep(1 * time.Second)
	err = store.SaveObservation("test-session", "Agent", "Obs 3")
	require.NoError(t, err)
	store.Close()

	// Commit 3: Iteration 3 (To be discarded)
	os.WriteFile(filepath.Join(workspace, "file1.txt"), []byte("v3"), 0644)
	gitClient.Run(workspace, "add", ".")
	gitClient.Run(workspace, "commit", "-m", "chore: progress update (iteration 3)")

	// Setup Session
	sm, err := runner.NewSessionManagerWithDir(sessionsDir)
	require.NoError(t, err)
	session := &runner.SessionState{
		Name:           "test-session",
		Workspace:      workspace,
		Status:         "stopped",
		AgentStateFile: filepath.Join(workspace, ".agent_state.json"),
	}
	sm.SaveSession(session)

	// Setup Agent State
	agentState := agent.State{
		Iteration: 3,
		History: []agent.Message{{Role: "user", Content: "foo"}},
	}
	asm := agent.NewStateManager(session.AgentStateFile)
	asm.Save(agentState)

	// Override Factories
	oldSMFactory := sessionManagerFactory
	oldGitFactory := gitClientFactory
	defer func() {
		sessionManagerFactory = oldSMFactory
		gitClientFactory = oldGitFactory
	}()

	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	gitClientFactory = func() IGitClient {
		return gitClient
	}

	// Execute Rewind to Iteration 2
	forceRewind = true
	err = rewindCmd.RunE(rewindCmd, []string{"test-session", "2"})
	require.NoError(t, err)

	// Verify Git State (Should be v2)
	content, _ := os.ReadFile(filepath.Join(workspace, "file1.txt"))
	assert.Equal(t, "v2", string(content))

	// Verify DB (Obs 3 should be gone)
	store, _ = db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	defer store.Close()
	history, _ := store.QueryHistory("test-session", 100)
	assert.Len(t, history, 1, "Should have 1 observation remaining")
	if len(history) > 0 {
		assert.Equal(t, "Obs 1", history[0].Content)
	}

	// Verify Agent State
	newState, _ := asm.Load()
	assert.Equal(t, 2, newState.Iteration)
}

// Mock Interfaces if needed (but we used real implementations in temp dir)
type mockSessionManager struct {
	ISessionManager
}
