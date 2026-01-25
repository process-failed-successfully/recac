package scenarios

import (
	"os/exec"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupGitRepo(t *testing.T) string {
	dir := t.TempDir()

	// Init git
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	err := cmd.Run()
	require.NoError(t, err)

	// Config user
	exec.Command("git", "-C", dir, "config", "user.email", "test@example.com").Run()
	exec.Command("git", "-C", dir, "config", "user.name", "Test User").Run()

	// Commit something to have master
	exec.Command("git", "-C", dir, "commit", "--allow-empty", "-m", "init").Run()

	return dir
}

func TestCheckAgentBranchExists(t *testing.T) {
	dir := setupGitRepo(t)

	// Create agent branch
	err := exec.Command("git", "-C", dir, "branch", "agent/test").Run()
	require.NoError(t, err)

	// Create a bare repo as remote
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()

	// Add remote
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	// Push master
	exec.Command("git", "-C", dir, "push", "origin", "master").Run()

	// Push agent branch
	exec.Command("git", "-C", dir, "push", "origin", "agent/test").Run()

	// Now check
	err = checkAgentBranchExists(dir)
	assert.NoError(t, err)
}

func TestCheckAgentBranchExists_Fail(t *testing.T) {
	dir := setupGitRepo(t)
	// No remote, or no agent branch on remote

	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()
	exec.Command("git", "-C", dir, "push", "origin", "master").Run()

	err := checkAgentBranchExists(dir)
	assert.Error(t, err)
}

func TestGetAgentBranch(t *testing.T) {
	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()
	exec.Command("git", "-C", dir, "push", "origin", "master").Run()

	exec.Command("git", "-C", dir, "branch", "agent/found").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/found").Run()

	branch, err := getAgentBranch(dir)
	require.NoError(t, err)
	assert.Equal(t, "agent/found", branch)
}

func TestGetSpecificAgentBranch(t *testing.T) {
	dir := setupGitRepo(t)
	remoteDir := t.TempDir()
	exec.Command("git", "init", "--bare", remoteDir).Run()
	exec.Command("git", "-C", dir, "remote", "add", "origin", remoteDir).Run()

	exec.Command("git", "-C", dir, "branch", "agent/T-123-fix").Run()
	exec.Command("git", "-C", dir, "push", "origin", "agent/T-123-fix").Run()

	branch, err := getSpecificAgentBranch(dir, "T-123")
	require.NoError(t, err)
	assert.Equal(t, "agent/T-123-fix", branch)

	_, err = getSpecificAgentBranch(dir, "T-999")
	assert.Error(t, err)
}
