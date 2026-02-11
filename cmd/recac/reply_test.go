package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/db"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReplyCmd(t *testing.T) {
	// 1. Setup Session Manager
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	// 2. Create a Workspace for the session
	workspace := t.TempDir()
	sessionName := "test-session"

	// 3. Create Session State File
	sessionState := &runner.SessionState{
		Name:      sessionName,
		Workspace: workspace,
		Status:    "running",
		StartTime: time.Now(),
		PID:       1234,
		LogFile:   filepath.Join(sm.SessionsDir(), sessionName+".log"),
	}

	// Manually write the session file because sm.StartSession spawns processes
	sessionData, err := json.Marshal(sessionState)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(sm.SessionsDir(), sessionName+".json"), sessionData, 0600)
	require.NoError(t, err)

	// 4. Setup DB in Workspace
	dbPath := filepath.Join(workspace, ".recac.db")
	store, err := db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})
	require.NoError(t, err)
	defer store.Close()

	// 5. Post a Question
	projectID := sessionName // reply.go logic assumes sessionName == projectID first
	err = store.SetSignal(projectID, "QUESTION", "What is the meaning of life?")
	require.NoError(t, err)

	// 6. Execute Reply Command (Answer provided)
	output, err := executeCommand(rootCmd, "reply", sessionName, "42")
	require.NoError(t, err)
	require.Contains(t, output, "Answer sent to agent")

	// 7. Verify Answer in DB
	answer, err := store.GetSignal(projectID, "ANSWER")
	require.NoError(t, err)
	require.Equal(t, "42", answer)
}

func TestReplyCmd_NoQuestion(t *testing.T) {
	sm, cleanup := setupTestSessionManager(t)
	defer cleanup()

	workspace := t.TempDir()
	sessionName := "test-session-no-q"

	sessionState := &runner.SessionState{
		Name:      sessionName,
		Workspace: workspace,
	}

	sessionData, _ := json.Marshal(sessionState)
	os.WriteFile(filepath.Join(sm.SessionsDir(), sessionName+".json"), sessionData, 0600)

	// Setup DB but NO question
	dbPath := filepath.Join(workspace, ".recac.db")
	db.NewStore(db.StoreConfig{Type: "sqlite", ConnectionString: dbPath})

	// Execute Reply
	_, err := executeCommand(rootCmd, "reply", sessionName, "42")
	require.Error(t, err)
	require.Contains(t, err.Error(), "no pending question found")
}

func TestReplyCmd_MissingSession(t *testing.T) {
	setupTestSessionManager(t)
	// No session created

	_, err := executeCommand(rootCmd, "reply", "non-existent", "42")
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to load session 'non-existent'")
}
