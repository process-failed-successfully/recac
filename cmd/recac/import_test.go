package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestImportCmd(t *testing.T) {
	// 1. Setup a temp directory for session manager
	tempDir := t.TempDir()

	// Create a mock SessionManager using the real one with a temp dir
	sm, err := runner.NewSessionManagerWithDir(tempDir)
	require.NoError(t, err)

	// Override the factory
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// 2. Create a dummy session state and log to zip
	sessionName := "test-session"
	session := &runner.SessionState{
		Name:      sessionName,
		Status:    "completed",
		StartTime: time.Now(),
		LogFile:   "placeholder/path", // This will be updated on import
	}

	metadataBytes, err := json.Marshal(session)
	require.NoError(t, err)

	logContent := []byte("This is a log file content")

	// Create zip file
	zipPath := filepath.Join(tempDir, "export.zip")
	zipFile, err := os.Create(zipPath)
	require.NoError(t, err)

	zipWriter := zip.NewWriter(zipFile)

	// Add metadata.json
	f, err := zipWriter.Create("metadata.json")
	require.NoError(t, err)
	_, err = f.Write(metadataBytes)
	require.NoError(t, err)

	// Add session.log
	f, err = zipWriter.Create("session.log")
	require.NoError(t, err)
	_, err = f.Write(logContent)
	require.NoError(t, err)

	err = zipWriter.Close()
	require.NoError(t, err)
	zipFile.Close()

	// 3. Run import command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err = importCmd.RunE(cmd, []string{zipPath})
	require.NoError(t, err)

	// 4. Verify output
	output := buf.String()
	assert.Contains(t, output, fmt.Sprintf("Successfully imported session '%s'", sessionName))

	// 5. Verify session exists in manager
	loadedSession, err := sm.LoadSession(sessionName)
	require.NoError(t, err)
	assert.Equal(t, sessionName, loadedSession.Name)
	assert.Contains(t, loadedSession.LogFile, sessionName+".log")

	// Verify log file exists
	logData, err := os.ReadFile(loadedSession.LogFile)
	require.NoError(t, err)
	assert.Equal(t, logContent, logData)
}

func TestImportCmd_Conflict(t *testing.T) {
	tempDir := t.TempDir()
	sm, err := runner.NewSessionManagerWithDir(tempDir)
	require.NoError(t, err)

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Pre-create a session with same name
	sessionName := "conflict-session"
	existingSession := &runner.SessionState{
		Name:      sessionName,
		Status:    "completed",
		StartTime: time.Now(),
		LogFile:   filepath.Join(tempDir, sessionName+".log"),
	}
	// We need to create the log file too, otherwise SaveSession/LoadSession might fail checks or cleanup logic
	err = os.WriteFile(existingSession.LogFile, []byte("existing logs"), 0600)
	require.NoError(t, err)

	err = sm.SaveSession(existingSession)
	require.NoError(t, err)

	// Create zip for the SAME session name
	session := &runner.SessionState{
		Name:      sessionName,
		Status:    "completed",
		StartTime: time.Now(),
	}
	metadataBytes, err := json.Marshal(session)
	require.NoError(t, err)

	zipPath := filepath.Join(tempDir, "import_conflict.zip")
	zipFile, err := os.Create(zipPath)
	require.NoError(t, err)
	zipWriter := zip.NewWriter(zipFile)
	f, _ := zipWriter.Create("metadata.json")
	f.Write(metadataBytes)
	f, _ = zipWriter.Create("session.log")
	f.Write([]byte("new logs"))
	zipWriter.Close()
	zipFile.Close()

	// Run import
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err = importCmd.RunE(cmd, []string{zipPath})
	require.NoError(t, err)

	// Verify renaming occurred
	output := buf.String()
	assert.Contains(t, output, "already exists. Renaming import to")
	assert.Contains(t, output, "Successfully imported session")

	// Verify both sessions exist
	_, err = sm.LoadSession(sessionName)
	require.NoError(t, err)

	// Find the new session
	sessions, err := sm.ListSessions()
	require.NoError(t, err)
	foundImported := false
	for _, s := range sessions {
		if s.Name != sessionName && strings.HasPrefix(s.Name, sessionName+"-imported-") {
			foundImported = true
			// Check logs content
			logData, _ := os.ReadFile(s.LogFile)
			assert.Equal(t, []byte("new logs"), logData)
		}
	}
	assert.True(t, foundImported, "Should have found renamed imported session")
}
