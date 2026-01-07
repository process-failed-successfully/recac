package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestRepo creates a temporary Git repository for testing.
func setupTestRepo(t *testing.T) (string, func()) {
	t.Helper()

	// Create a temporary directory for the repository
	repoPath, err := os.MkdirTemp("", "recac-test-repo-*")
	require.NoError(t, err, "Failed to create temp dir for test repo")

	// Teardown function to remove the repository after the test
	cleanup := func() {
		os.RemoveAll(repoPath)
	}

	// Helper function to run commands in the repo
	runCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Git command failed: git %s\nOutput: %s", strings.Join(args, " "), string(output))
	}

	// Initialize the repository
	runCmd("init", "--initial-branch=main")
	runCmd("config", "user.name", "Test User")
	runCmd("config", "user.email", "test@example.com")

	// Create an initial commit
	err = os.WriteFile(filepath.Join(repoPath, "README.md"), []byte("initial commit"), 0644)
	require.NoError(t, err)
	runCmd("add", "README.md")
	runCmd("commit", "-m", "Initial commit")

	// Create feature branches
	runCmd("checkout", "-b", "feature/test-feature-1")
	err = os.WriteFile(filepath.Join(repoPath, "feature1.txt"), []byte("content1"), 0644)
	require.NoError(t, err)
	runCmd("add", "feature1.txt")
	runCmd("commit", "-m", "feat: implement feature 1")

	runCmd("checkout", "main")
	runCmd("checkout", "-b", "feature/test-feature-2")
	err = os.WriteFile(filepath.Join(repoPath, "feature2.txt"), []byte("content2"), 0644)
	require.NoError(t, err)
	runCmd("add", "feature2.txt")
	runCmd("commit", "-m", "feat: implement feature 2")

	// Create a non-feature branch
	runCmd("checkout", "main")
	runCmd("checkout", "-b", "fix/bug-fix-branch")
	err = os.WriteFile(filepath.Join(repoPath, "fix.txt"), []byte("fix"), 0644)
	require.NoError(t, err)
	runCmd("add", "fix.txt")
	runCmd("commit", "-m", "fix: a bug")

	// Go back to main
	runCmd("checkout", "main")

	// Set the working directory to the repo path for the test
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	err = os.Chdir(repoPath)
	require.NoError(t, err)

	// The full cleanup function now also restores the original working directory
	fullCleanup := func() {
		os.Chdir(originalWd)
		cleanup()
	}

	return repoPath, fullCleanup
}

func TestListFeatures(t *testing.T) {
	_, cleanup := setupTestRepo(t)
	defer cleanup()

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run the function
	err := listFeatures()
	require.NoError(t, err)

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	output := buf.String()

	// Assertions
	assert.Contains(t, output, "FEATURE")
	assert.Contains(t, output, "AUTHOR")
	assert.Contains(t, output, "LAST COMMIT")
	assert.Contains(t, output, "MESSAGE")

	assert.Contains(t, output, "test-feature-1")
	assert.Contains(t, output, "feat: implement feature 1")

	assert.Contains(t, output, "test-feature-2")
	assert.Contains(t, output, "feat: implement feature 2")

	assert.NotContains(t, output, "fix/bug-fix-branch")
	assert.NotContains(t, output, "main")
}

func TestListFeatures_NoFeatures(t *testing.T) {
	// Setup a repo with no feature branches
	repoPath, cleanup := setupTestRepo(t)
	defer cleanup()

	// Helper to run commands
	runCmd := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoPath
		output, err := cmd.CombinedOutput()
		require.NoError(t, err, "Git command failed: git %s\nOutput: %s", strings.Join(args, " "), string(output))
	}

	// Delete feature branches
	runCmd("branch", "-D", "feature/test-feature-1")
	runCmd("branch", "-D", "feature/test-feature-2")


	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Run the function
	err := listFeatures()
	require.NoError(t, err)

	// Restore stdout and read captured output
	w.Close()
	os.Stdout = oldStdout
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)

	output := buf.String()

	// Assertions
	assert.Contains(t, output, "No features found.")
	assert.NotContains(t, output, "FEATURE")
}
