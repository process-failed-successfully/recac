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
	// Temporarily override the session manager factory
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
}

func TestWorkdiffCmd_TwoSessions(t *testing.T) {
	// 1. Setup a temporary git repository
	repoDir, err := os.MkdirTemp("", "recac-workdiff-two-session-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(repoDir)

	runCmd := func(args ...string) (string, error) {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = repoDir
		output, err := cmd.CombinedOutput()
		return string(output), err
	}

	_, err = runCmd("git", "init", "-b", "main")
	require.NoError(t, err)
	_, err = runCmd("git", "config", "user.email", "test@example.com")
	require.NoError(t, err)
	_, err = runCmd("git", "config", "user.name", "Test User")
	require.NoError(t, err)

	// 2. Create commits
	getCommitHash := func() string {
		out, err := runCmd("git", "rev-parse", "HEAD")
		require.NoError(t, err)
		return strings.TrimSpace(out)
	}

	// Commit A (Base)
	err = os.WriteFile(filepath.Join(repoDir, "base.txt"), []byte("base file"), 0644)
	require.NoError(t, err)
	_, err = runCmd("git", "add", ".")
	require.NoError(t, err)
	_, err = runCmd("git", "commit", "-m", "base commit")
	require.NoError(t, err)
	commitA := getCommitHash()

	// Commit B (End of Session A)
	err = os.WriteFile(filepath.Join(repoDir, "session_a_work.txt"), []byte("work from session a"), 0644)
	require.NoError(t, err)
	_, err = runCmd("git", "add", ".")
	require.NoError(t, err)
	_, err = runCmd("git", "commit", "-m", "session a commit")
	require.NoError(t, err)
	commitB := getCommitHash()

	// Go back to base and create a different commit for session B
	_, err = runCmd("git", "reset", "--hard", commitA)
	require.NoError(t, err)

	// Commit C (End of Session B)
	err = os.WriteFile(filepath.Join(repoDir, "session_b_work.txt"), []byte("work from session b"), 0644)
	require.NoError(t, err)
	_, err = runCmd("git", "add", ".")
	require.NoError(t, err)
	_, err = runCmd("git", "commit", "-m", "session b commit")
	require.NoError(t, err)
	commitC := getCommitHash()

	// 3. Create mock sessions
	sessionsDir, err := os.MkdirTemp("", "recac-sessions-*")
	require.NoError(t, err)
	defer os.RemoveAll(sessionsDir)

	sm, err := runner.NewSessionManagerWithDir(sessionsDir)
	require.NoError(t, err)

	sessionA := &runner.SessionState{
		Name:           "session-a",
		Status:         "completed",
		Workspace:      repoDir,
		StartCommitSHA: commitA,
		EndCommitSHA:   commitB,
	}
	err = sm.SaveSession(sessionA)
	require.NoError(t, err)

	sessionB := &runner.SessionState{
		Name:           "session-b",
		Status:         "completed",
		Workspace:      repoDir,
		StartCommitSHA: commitA,
		EndCommitSHA:   commitC,
	}
	err = sm.SaveSession(sessionB)
	require.NoError(t, err)

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
	require.Contains(t, output, "diff --git a/session_a_work.txt b/session_a_work.txt")
	require.Contains(t, output, "deleted file mode 100644")
	require.Contains(t, output, "--- a/session_a_work.txt")
	require.Contains(t, output, "+++ /dev/null")
	require.Contains(t, output, "-work from session a")

	require.Contains(t, output, "diff --git a/session_b_work.txt b/session_b_work.txt")
	require.Contains(t, output, "new file mode 100644")
	require.Contains(t, output, "--- /dev/null")
	require.Contains(t, output, "+++ b/session_b_work.txt")
	require.Contains(t, output, "+work from session b")
}
