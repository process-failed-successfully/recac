package main

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSecurityHistoryCmd(t *testing.T) {
	// Setup temp directory
	tempDir, err := os.MkdirTemp("", "recac-security-history-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Change to temp dir
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	// Initialize git repo
	runGit(t, "init")
	runGit(t, "config", "user.email", "test@example.com")
	runGit(t, "config", "user.name", "Test User")

	// Commit 1: Clean file
	os.WriteFile("main.go", []byte("package main\nfunc main() {}\n"), 0644)
	runGit(t, "add", ".")
	runGit(t, "commit", "-m", "Initial commit")

	// Commit 2: Add Secret
	secretContent := "aws_key = \"AKIA1234567890123456\"\n" // Matches AKIA[0-9A-Z]{16}
	os.WriteFile("config.txt", []byte(secretContent), 0644)
	runGit(t, "add", ".")
	runGit(t, "commit", "-m", "Add secret")

	// Commit 3: Remove Secret
	os.WriteFile("config.txt", []byte("aws_key = \"REDACTED\"\n"), 0644)
	runGit(t, "add", ".")
	runGit(t, "commit", "-m", "Remove secret")

	// Helper to reset flags
	resetHistoryFlags := func() {
		securityHistoryCommits = 50
		securityHistoryAll = false
		securityHistoryJSON = false
		securityHistoryFail = false
	}

	t.Run("Find Secret In History", func(t *testing.T) {
		resetHistoryFlags()
		cmd := securityHistoryCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer)) // Ignore stderr

		err := cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "AWS Access Key")
		assert.Contains(t, output, "config.txt")
	})

	t.Run("JSON Output", func(t *testing.T) {
		resetHistoryFlags()
		securityHistoryJSON = true
		cmd := securityHistoryCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		var results []HistorySecurityResult
		err = json.Unmarshal(buf.Bytes(), &results)
		require.NoError(t, err)

		assert.NotEmpty(t, results)
		found := false
		for _, r := range results {
			if r.Type == "AWS Access Key" && r.File == "config.txt" {
				found = true
				break
			}
		}
		assert.True(t, found, "Should find AWS Key in config.txt")
	})

	t.Run("Fail Flag", func(t *testing.T) {
		resetHistoryFlags()
		securityHistoryFail = true
		cmd := securityHistoryCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "found")
		assert.Contains(t, err.Error(), "secrets in history")
	})

	t.Run("Limit Commits", func(t *testing.T) {
		resetHistoryFlags()
		securityHistoryCommits = 1 // Only look at last 1 commit (Remove secret)
		// The secret was added in Commit 2 (2 commits ago).
		// So checking last 1 commit should NOT find it.
		// Wait, Commit 3 is "Remove secret". Commit 2 is "Add secret". Commit 1 is "Initial".
		// Last 1 commit is Commit 3.
		// So it should be clean.

		cmd := securityHistoryCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		require.NoError(t, err)
		assert.Contains(t, buf.String(), "No secrets found")
	})
}

func runGit(t *testing.T, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err := cmd.Run()
	require.NoError(t, err, "git %v failed", args)
}
