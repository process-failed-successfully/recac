package main

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"recac/internal/runner"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestExportCmd(t *testing.T) {
	// 1. Setup Mock Session
	tmpDir := t.TempDir()
	t.Setenv("HOME", tmpDir)

	sm, err := runner.NewSessionManagerWithDir(filepath.Join(tmpDir, ".recac", "sessions"))
	require.NoError(t, err)

	sessionName := "test-session"
	session := &runner.SessionState{
		Name:           sessionName,
		Status:         "completed",
		StartTime:      time.Now(),
		EndTime:        time.Now().Add(5 * time.Minute),
		StartCommitSHA: "mock_start_sha",
		EndCommitSHA:   "mock_end_sha",
		Workspace:      "/tmp/workspace",
		LogFile:        filepath.Join(sm.SessionsDir(), sessionName+".log"),
	}

	// Create a dummy log file
	logContent := "This is a log line."
	err = os.WriteFile(session.LogFile, []byte(logContent), 0600)
	require.NoError(t, err)

	err = sm.SaveSession(session)
	require.NoError(t, err)

	// Override factory to ensure command uses our session manager
	originalFactory := sessionManagerFactory
	defer func() { sessionManagerFactory = originalFactory }()
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}

	// 2. Setup Mock Git Client
	originalGitFactory := gitClientFactory
	defer func() { gitClientFactory = originalGitFactory }()
	gitClientFactory = func() IGitClient {
		return &MockGitClient{
			DiffFunc: func(workspace, fromSHA, toSHA string) (string, error) {
				return "mock diff content", nil
			},
		}
	}

	// 3. Execute the Command
	outputDir := t.TempDir()
	outputPath := filepath.Join(outputDir, "test.zip")
	rootCmd, _, _ := newRootCmd()
	rootCmd.SetArgs([]string{"export", sessionName, "--output", outputPath})
	err = rootCmd.Execute()
	require.NoError(t, err)

	// 4. Verify the Zip Archive
	r, err := zip.OpenReader(outputPath)
	require.NoError(t, err)
	defer r.Close()

	// Check for metadata.json
	f, err := r.Open("metadata.json")
	require.NoError(t, err)
	metaContent, err := io.ReadAll(f)
	require.NoError(t, err)
	f.Close()
	require.Contains(t, string(metaContent), `"name": "test-session"`)

	// Check for session.log
	f, err = r.Open("session.log")
	require.NoError(t, err)
	logFileContent, err := io.ReadAll(f)
	require.NoError(t, err)
	f.Close()
	require.Equal(t, logContent, string(logFileContent))

	// Check for work.diff
	f, err = r.Open("work.diff")
	require.NoError(t, err)
	diffFileContent, err := io.ReadAll(f)
	require.NoError(t, err)
	f.Close()
	require.Equal(t, "mock diff content", string(diffFileContent))
}
