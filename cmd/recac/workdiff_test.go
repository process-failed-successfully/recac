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

func TestWorkdiffCmd(t *testing.T) {
	t.Run("single session", func(t *testing.T) {
		// 1. Setup a temporary git repository
		repoDir, err := os.MkdirTemp("", "recac-workdiff-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(repoDir)

		runCmd := func(args ...string) {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = repoDir
			err := cmd.Run()
			require.NoError(t, err, "failed to run git command: %v", args)
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
		defer os.RemoveAll(sessionsDir)

		sm, err := runner.NewSessionManagerWithDir(sessionsDir)
		require.NoError(t, err)

		sessionName := "workdiff-test-session"
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

		// 5. Run the workdiff command
		rootCmd, _, _ := newRootCmd()
		originalFactory := sessionManagerFactory
		sessionManagerFactory = func() (ISessionManager, error) {
			return sm, nil
		}
		defer func() { sessionManagerFactory = originalFactory }()

		output, err := executeCommand(rootCmd, "workdiff", sessionName)
		require.NoError(t, err)

		// 6. Assert the output
		require.Contains(t, output, "diff --git a/test.txt b/test.txt")
		require.Contains(t, output, "-hello")
		require.Contains(t, output, "+hello world")
	})

	t.Run("two sessions", func(t *testing.T) {
		// 1. Setup a temporary git repository
		repoDir, err := os.MkdirTemp("", "recac-workdiff-two-session-test-*")
		require.NoError(t, err)
		defer os.RemoveAll(repoDir)

		runCmd := func(args ...string) {
			cmd := exec.Command(args[0], args[1:]...)
			cmd.Dir = repoDir
			err := cmd.Run()
			require.NoError(t, err, "failed to run git command: %v", args)
		}

		getCommit := func() string {
			cmd := exec.Command("git", "rev-parse", "HEAD")
			cmd.Dir = repoDir
			bytes, err := cmd.Output()
			require.NoError(t, err)
			return strings.TrimSpace(string(bytes))
		}

		runCmd("git", "init", "-b", "main")
		runCmd("git", "config", "user.email", "test@example.com")
		runCmd("git", "config", "user.name", "Test User")

		// 2. Create commits
		err = os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("commit one"), 0644)
		require.NoError(t, err)
		runCmd("git", "add", ".")
		runCmd("git", "commit", "-m", "commit 1")

		err = os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("commit two"), 0644)
		require.NoError(t, err)
		runCmd("git", "add", ".")
		runCmd("git", "commit", "-m", "commit 2")
		commitTwoSHA := getCommit()

		err = os.WriteFile(filepath.Join(repoDir, "test.txt"), []byte("commit three"), 0644)
		require.NoError(t, err)
		runCmd("git", "add", ".")
		runCmd("git", "commit", "-m", "commit 3")
		commitThreeSHA := getCommit()

		// 3. Create mock sessions
		sessionsDir, err := os.MkdirTemp("", "recac-sessions-two-*")
		require.NoError(t, err)
		defer os.RemoveAll(sessionsDir)

		sm, err := runner.NewSessionManagerWithDir(sessionsDir)
		require.NoError(t, err)

		sessionA := &runner.SessionState{
			Name:         "session-a",
			Status:       "completed",
			Workspace:    repoDir,
			EndCommitSHA: commitTwoSHA,
		}
		require.NoError(t, sm.SaveSession(sessionA))

		sessionB := &runner.SessionState{
			Name:         "session-b",
			Status:       "completed",
			Workspace:    repoDir,
			EndCommitSHA: commitThreeSHA,
		}
		require.NoError(t, sm.SaveSession(sessionB))

		// 4. Run the workdiff command
		rootCmd, _, _ := newRootCmd()
		originalFactory := sessionManagerFactory
		sessionManagerFactory = func() (ISessionManager, error) {
			return sm, nil
		}
		defer func() { sessionManagerFactory = originalFactory }()

		output, err := executeCommand(rootCmd, "workdiff", "session-a", "session-b")
		require.NoError(t, err)

		// 5. Assert the output
		require.Contains(t, output, "diff --git a/test.txt b/test.txt")
		require.Contains(t, output, "-commit two")
		require.Contains(t, output, "+commit three")
	})
}
