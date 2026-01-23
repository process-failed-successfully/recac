package runner

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGitSafeguard_UntrackFiles(t *testing.T) {
	// Setup git repo
	tmpDir := t.TempDir()
	exec.Command("git", "-C", tmpDir, "init").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", tmpDir, "config", "user.name", "Test").Run()

	// Create a file and track it
	file := "state.json"
	path := filepath.Join(tmpDir, file)
	os.WriteFile(path, []byte("{}"), 0644)

	err := exec.Command("git", "-C", tmpDir, "add", file).Run()
	require.NoError(t, err)

	err = exec.Command("git", "-C", tmpDir, "commit", "-m", "add state").Run()
	require.NoError(t, err)

	// Verify it is tracked
	cmd := exec.Command("git", "-C", tmpDir, "ls-files", file)
	out, _ := cmd.Output()
	assert.Contains(t, string(out), file)

	// Run untrackFiles (using private function via export_test.go or assuming we are in package)
	// We are in package runner.
	err = untrackFiles(tmpDir, []string{file})
	assert.NoError(t, err)

	// Verify it is untracked (removed from index)
	cmd = exec.Command("git", "-C", tmpDir, "ls-files", file)
	out, _ = cmd.Output()
	assert.Empty(t, string(out))

	// File should still exist on disk
	_, err = os.Stat(path)
	assert.NoError(t, err)
}

func TestGitSafeguard_EnsureStateIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	exec.Command("git", "-C", tmpDir, "init").Run()

	// Create state file
	stateFile := ".agent_state.json"
	path := filepath.Join(tmpDir, stateFile)
	os.WriteFile(path, []byte("{}"), 0644)

	// Ensure ignored
	err := EnsureStateIgnored(tmpDir)
	assert.NoError(t, err)

	// Verify .gitignore
	gitignore := filepath.Join(tmpDir, ".gitignore")
	content, err := os.ReadFile(gitignore)
	assert.NoError(t, err)
	assert.Contains(t, string(content), stateFile)
}
