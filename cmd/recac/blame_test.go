package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseBlameOutput(t *testing.T) {
	// Sample output from git blame --line-porcelain
	output := `d6a04005b642407511475654308569528659543e 1 1 1
author Jules
author-mail <jules@example.com>
author-time 1700000000
author-tz +0000
committer Jules
committer-mail <jules@example.com>
committer-time 1700000000
committer-tz +0000
summary Initial commit
boundary
filename test.txt
	Hello World
`
	lines, err := parseBlameOutput([]byte(output))
	require.NoError(t, err)
	require.Len(t, lines, 1)

	assert.Equal(t, "d6a04005b642407511475654308569528659543e", lines[0].SHA)
	assert.Equal(t, "Jules", lines[0].Author)
	assert.Equal(t, "Initial commit", lines[0].Summary)
	assert.Equal(t, "Hello World", lines[0].Content)
	// Check date conversion
	expectedDate := time.Unix(1700000000, 0).Format("2006-01-02")
	assert.Equal(t, expectedDate, lines[0].Date)
}

func TestBlameCommand_Integration(t *testing.T) {
	// Ensure git is available
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not found")
	}

	// Setup temp repo
	dir := t.TempDir()

	// git init
	runGitCmdForTest(t, dir, "git", "init")
	// Configure git for test
	runGitCmdForTest(t, dir, "git", "config", "user.name", "Test User")
	runGitCmdForTest(t, dir, "git", "config", "user.email", "test@example.com")

	// Create file
	filePath := filepath.Join(dir, "hello.txt")
	err := os.WriteFile(filePath, []byte("Hello\nWorld"), 0644)
	require.NoError(t, err)

	runGitCmdForTest(t, dir, "git", "add", "hello.txt")
	runGitCmdForTest(t, dir, "git", "commit", "-m", "First commit")

	// Test executeGitBlame
	// executeGitBlame uses exec.Command, which runs in CWD of the process.
	// But `git blame` on an absolute path works IF we are inside the repo or if git can find the repo.
	// The test process CWD is likely the package dir.
	// So `git blame /tmp/repo/hello.txt` might fail if CWD is not inside /tmp/repo.
	// We need to change CWD or use `git -C <dir> blame`.

	// Since executeGitBlame hardcodes `git blame ... file`, and we can't easily change CWD safely in parallel tests.
	// BUT we can use `blameExecCommand` override to force the directory!

	origExec := blameExecCommand
	defer func() { blameExecCommand = origExec }()

	blameExecCommand = func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir // Force execution in the repo dir
		return cmd
	}

	// We pass just the filename relative to dir? Or absolute?
	// If we use absolute path in args, git blame works even if CWD is outside, USUALLY.
	// Let's try passing just the filename and rely on CWD being set to dir.

	output, err := executeGitBlame("hello.txt")
	require.NoError(t, err)

	lines, err := parseBlameOutput(output)
	require.NoError(t, err)
	require.Len(t, lines, 2)

	assert.Equal(t, "Hello", lines[0].Content)
	assert.Equal(t, "World", lines[1].Content)
	assert.Equal(t, "Test User", lines[0].Author)
}

func runGitCmdForTest(t *testing.T, dir, name string, args ...string) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Command %s %v failed: %v\nOutput: %s", name, args, err, out)
	}
}
