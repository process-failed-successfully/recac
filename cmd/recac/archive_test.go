package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"recac/internal/runner"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchiveCmd(t *testing.T) {
	// Setup: Create a temporary directory for sessions and output
	tempDir := t.TempDir()
	sessionsDir := filepath.Join(tempDir, "sessions")
	err := os.Mkdir(sessionsDir, 0755)
	require.NoError(t, err)

	// Setup: Create mock sessions
	mockSessions := map[string]*runner.SessionState{
		"session1": {
			Name:      "session1",
			LogFile:   filepath.Join(sessionsDir, "session1.log"),
			StartTime: time.Now().Add(-2 * time.Hour),
		},
		"session2": {
			Name:      "session2",
			LogFile:   filepath.Join(sessionsDir, "session2.log"),
			StartTime: time.Now().Add(-1 * time.Hour),
		},
		"session3-no-log": {
			Name:      "session3-no-log",
			LogFile:   filepath.Join(sessionsDir, "session3-no-log.log"), // File won't be created
			StartTime: time.Now(),
		},
	}

	// Create dummy log files
	err = os.WriteFile(mockSessions["session1"].LogFile, []byte("log content for session1"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(mockSessions["session2"].LogFile, []byte("log content for session2"), 0644)
	require.NoError(t, err)

	// Setup: Mock session manager
	mockSM := &MockSessionManager{
		Sessions: mockSessions,
	}
	sessionManagerFactory = func() (ISessionManager, error) {
		return mockSM, nil
	}

	t.Run("archive multiple specified sessions", func(t *testing.T) {
		outputZip := filepath.Join(tempDir, "archive_multi.zip")
		rootCmd, _, _ := newRootCmd()
		output, err := executeCommand(rootCmd, "archive", "session1", "session2", "--output", outputZip)

		require.NoError(t, err)
		assert.Contains(t, output, "Successfully archived 2 sessions")

		// Verify zip contents
		expectedFiles := []string{
			"session1/metadata.json",
			"session1/session.log",
			"session2/metadata.json",
			"session2/session.log",
		}
		verifyZipContents(t, outputZip, expectedFiles)
	})

	t.Run("archive all sessions with --all flag", func(t *testing.T) {
		outputZip := filepath.Join(tempDir, "archive_all.zip")
		rootCmd, _, _ := newRootCmd()
		output, err := executeCommand(rootCmd, "archive", "--all", "--output", outputZip)

		require.NoError(t, err)
		assert.Contains(t, output, fmt.Sprintf("Successfully archived %d sessions", len(mockSessions)))

		// Verify zip contents - session3 will be skipped because its log file doesn't exist
		expectedFiles := []string{
			"session1/metadata.json",
			"session1/session.log",
			"session2/metadata.json",
			"session2/session.log",
			"session3-no-log/metadata.json",
		}
		verifyZipContents(t, outputZip, expectedFiles)
	})

	t.Run("fail if no session name is provided without --all flag", func(t *testing.T) {
		rootCmd, _, _ := newRootCmd()
		_, err := executeCommand(rootCmd, "archive")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "you must specify at least one session name or use the --all flag")
	})

	t.Run("fail if a specified session does not exist", func(t *testing.T) {
		rootCmd, _, _ := newRootCmd()
		_, err := executeCommand(rootCmd, "archive", "session1", "non-existent-session")

		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to load session non-existent-session")
	})

	t.Run("succeed with message when no sessions exist and --all is used", func(t *testing.T) {
		// Override mock to return no sessions
		sessionManagerFactory = func() (ISessionManager, error) {
			return &MockSessionManager{Sessions: map[string]*runner.SessionState{}}, nil
		}
		defer func() { // Restore
			sessionManagerFactory = func() (ISessionManager, error) { return mockSM, nil }
		}()

		rootCmd, _, _ := newRootCmd()
		output, err := executeCommand(rootCmd, "archive", "--all")

		require.NoError(t, err)
		assert.Contains(t, output, "No sessions found to archive")
	})
}

// verifyZipContents is a helper to check if a zip archive contains the expected files.
func verifyZipContents(t *testing.T, zipPath string, expectedFiles []string) {
	t.Helper()

	r, err := zip.OpenReader(zipPath)
	require.NoError(t, err)
	defer r.Close()

	foundFiles := make(map[string]bool)
	for _, f := range r.File {
		// Use filepath.ToSlash to handle OS-independent paths
		foundFiles[filepath.ToSlash(f.Name)] = true
	}

	for _, expected := range expectedFiles {
		// Also normalize expected file path
		normalizedExpected := strings.ReplaceAll(expected, string(os.PathSeparator), "/")
		assert.True(t, foundFiles[normalizedExpected], "expected file not found in zip: %s", normalizedExpected)
	}
	assert.Equal(t, len(expectedFiles), len(foundFiles), "zip file contains an unexpected number of files")
}
