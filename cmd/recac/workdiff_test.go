package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/runner"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

// setupWorkdiffTest is a helper to create a git repo and a mock session for testing.
func setupWorkdiffTest(t *testing.T) (sm *runner.SessionManager, sessionName string, repoDir string, startCommit string, endCommit string) {
	t.Helper()

	// 1. Setup a temporary git repository
	repoDir, err := os.MkdirTemp("", "recac-workdiff-test-*")
	require.NoError(t, err)

	runCmd := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run git command: %v, output: %s", args, output)
		}
	}

	runCmd("git", "init", "-b", "main")
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
	startCommit = strings.TrimSpace(string(startCommitBytes))

	// 3. Create the second commit
	err = os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("hello world"), 0644)
	require.NoError(t, err)
	runCmd("git", "add", ".")
	runCmd("git", "commit", "-m", "second commit")

	endCommitCmd := exec.Command("git", "rev-parse", "HEAD")
	endCommitCmd.Dir = repoDir
	endCommitBytes, err := endCommitCmd.Output()
	require.NoError(t, err)
	endCommit = strings.TrimSpace(string(endCommitBytes))

	// 4. Create a mock session
	sessionsDir, err := os.MkdirTemp("", "recac-sessions-*")
	require.NoError(t, err)
	t.Cleanup(func() { os.RemoveAll(sessionsDir) })

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

	return sm, sessionName, repoDir, startCommit, endCommit
}

func TestWorkdiffCmd(t *testing.T) {
	sm, sessionName, repoDir, _, _ := setupWorkdiffTest(t)
	defer os.RemoveAll(repoDir)

	rootCmd := &cobra.Command{Use: "recac"}
	rootCmd.AddCommand(workdiffCmd)

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	output, err := executeCommand(rootCmd, "workdiff", sessionName)
	require.NoError(t, err)

	require.Contains(t, output, "diff --git a/test.txt b/test.txt")
	require.Contains(t, output, "-hello")
	require.Contains(t, output, "+hello world")
}

func TestShowAliasCmd(t *testing.T) {
	sm, sessionName, repoDir, _, _ := setupWorkdiffTest(t)
	defer os.RemoveAll(repoDir)

	rootCmd := &cobra.Command{Use: "recac"}
	rootCmd.AddCommand(workdiffCmd)

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

func TestWorkdiffTwoSessions(t *testing.T) {
	sm, session1Name, repoDir, startCommit, _ := setupWorkdiffTest(t)
	defer os.RemoveAll(repoDir)

	// Create a third commit for the second session
	runCmd := func(args ...string) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("failed to run git command: %v, output: %s", args, output)
		}
	}

	err := os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("hello world again"), 0644)
	require.NoError(t, err)
	runCmd("git", "add", ".")
	runCmd("git", "commit", "-m", "third commit")

	endCommitCmd := exec.Command("git", "rev-parse", "HEAD")
	endCommitCmd.Dir = repoDir
	endCommitBytes, err := endCommitCmd.Output()
	require.NoError(t, err)
	endCommit2 := strings.TrimSpace(string(endCommitBytes))

	// Create second session
	session2Name := "session-2"
	session2 := &runner.SessionState{
		Name:           session2Name,
		Status:         "completed",
		StartTime:      time.Now(),
		EndTime:        time.Now(),
		Workspace:      repoDir,
		StartCommitSHA: startCommit,
		EndCommitSHA:   endCommit2,
	}
	err = sm.SaveSession(session2)
	require.NoError(t, err)

	rootCmd := &cobra.Command{Use: "recac"}
	rootCmd.AddCommand(workdiffCmd)

	originalFactory := sessionManagerFactory
	sessionManagerFactory = func() (ISessionManager, error) {
		return sm, nil
	}
	defer func() { sessionManagerFactory = originalFactory }()

	// Execute workdiff session1 session2
	// Expected diff: endCommit1 (hello world) -> endCommit2 (hello world again)
	output, err := executeCommand(rootCmd, "workdiff", session1Name, session2Name)
	require.NoError(t, err)

	require.Contains(t, output, "diff --git a/test.txt b/test.txt")
	require.Contains(t, output, "-hello world")
	require.Contains(t, output, "+hello world again")
}

func TestGetSessionEndSHA_Cases(t *testing.T) {
	sm, sessionName, repoDir, _, _ := setupWorkdiffTest(t)
	defer os.RemoveAll(repoDir)

	t.Run("with explicit EndCommitSHA", func(t *testing.T) {
		session, err := sm.LoadSession(sessionName)
		require.NoError(t, err)
		session.EndCommitSHA = "abcdef123"

		sha, err := getSessionEndSHA(session)
		require.NoError(t, err)
		require.Equal(t, "abcdef123", sha)
	})

	t.Run("without EndCommitSHA but completed", func(t *testing.T) {
		session, err := sm.LoadSession(sessionName)
		require.NoError(t, err)
		session.EndCommitSHA = "" // Clear it
		session.Status = "completed"
        session.Workspace = repoDir

		sha, err := getSessionEndSHA(session)
		require.NoError(t, err)
		require.NotEmpty(t, sha) // Should get HEAD from repo
	})

	t.Run("without EndCommitSHA and running", func(t *testing.T) {
		session, err := sm.LoadSession(sessionName)
		require.NoError(t, err)
		session.EndCommitSHA = ""
		session.Status = "running"

		_, err = getSessionEndSHA(session)
		require.Error(t, err)
		require.Contains(t, err.Error(), "is still running")
	})
}
