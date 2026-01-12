package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// setupWorkdiffTest is a helper to create a git repo and a mock session for testing.
func setupWorkdiffTest(t *testing.T) (sm *runner.SessionManager, sessionName, repoDir string) {
	t.Helper()

	// 1. Setup a temporary git repository
	repoDir, err := os.MkdirTemp("", "recac-workdiff-test-*")
	require.NoError(t, err)

	runCmd := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		err := cmd.Run()
		require.NoError(t, err, "failed to run git command: %v", args)
	}

	runCmd("git", "init", "-b", "main") // Specify default branch
	runCmd("git", "config", "user.email", "test@example.com")
	runCmd("git", "config", "user.name", "Test User")

	// 2. Create the first commit
	err = os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("hello"), 0644)
	require.NoError(t, err)
	runCmd("git", "add", ".")
	runCmd("git", "commit", "-m", "initial commit")
	startCommitCmd := exec.Command("git", "rev-parse", "HEAD")
	startCommitCmd.Dir = repoDir
	startCommitBytes, err := startCommitCmd.Output()
	require.NoError(t, err)
	startCommit := strings.TrimSpace(string(startCommitBytes))

	// 3. Create the second commit
	err = os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("hello world"), 0644)
	require.NoError(t, err)
	runCmd("git", "add", ".")
	runCmd("git", "commit", "-m", "second commit")
	endCommitCmd := exec.Command("git", "rev-parse", "HEAD")
	endCommitCmd.Dir = repoDir
	endCommitBytes, err := endCommitCmd.Output()
	require.NoError(t, err)
	endCommit := strings.TrimSpace(string(endCommitBytes))

	// 4. Create a mock session
	sessionsDir, err := os.MkdirTemp("", "recac-sessions-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(sessionsDir) }) // Clean up sessions dir

	sm, err = runner.NewSessionManagerWithDir(sessionsDir)
	require.NoError(t, err)

	sessionName = "workdiff-test-session"
	session := &runner.SessionState{
		Name:           sessionName,
		Status:         "completed",
		StartTime:      time.Now(),
		EndTime:        time.Now(),
		Workspace:      repoDir,
		StartCommitSHA: startCommit,
		EndCommitSHA:   endCommit,
	}
	err = sm.SaveSession(session)
	require.NoError(t, err)

	return sm, sessionName, repoDir
}

func TestWorkdiffCmd(t *testing.T) {
	sm, sessionName, repoDir := setupWorkdiffTest(t)
	defer os.RemoveAll(repoDir)

	// Run the workdiff command
	rootCmd, _, _ := newRootCmd()
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	output, err := executeCommand(rootCmd, "workdiff", sessionName)
	require.NoError(t, err)

	// Assert the output
	require.Contains(t, output, "diff --git a/test.txt b/test.txt")
	require.Contains(t, output, "-hello")
	require.Contains(t, output, "+hello world")
}

func TestShowAliasCmd(t *testing.T) {
	sm, sessionName, repoDir := setupWorkdiffTest(t)
	defer os.RemoveAll(repoDir)

	rootCmd, _, _ := newRootCmd()
	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	t.Run("show with one argument should succeed", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "show", sessionName)
		require.NoError(t, err)
		require.Contains(t, output, "diff --git a/test.txt b/test.txt")
		require.Contains(t, output, "-hello")
		require.Contains(t, output, "+hello world")
	})

	t.Run("show with two arguments should fail", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "show", sessionName, "another-session")
		require.Error(t, err)
		require.Equal(t, "the 'show' alias requires exactly one session name", err.Error())
	})

	t.Run("show with no arguments should fail", func(t *testing.T) {
		_, err := executeCommand(rootCmd, "show")
		require.Error(t, err)
		require.Equal(t, "the 'show' alias requires exactly one session name", err.Error())
	})
}
