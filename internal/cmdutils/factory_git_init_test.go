package cmdutils

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/git"

	"github.com/stretchr/testify/assert"
)

func TestSetupWorkspace_SkipRepo_Init(t *testing.T) {
	// Create temp workspace
	workspace, err := os.MkdirTemp("", "repro-test-*")
	assert.NoError(t, err)
	defer os.RemoveAll(workspace)

	// Create real git client
	gitClient := git.NewClient()

	// Call SetupWorkspace with repoURL="skip"
	ctx := context.Background()
	_, err = SetupWorkspace(ctx, gitClient, "skip", workspace, "TEST-123", "", "")
	assert.NoError(t, err)

	// Verify workspace is a git repo
	isRepo := gitClient.RepoExists(workspace)
	assert.True(t, isRepo, "Workspace should be initialized as git repo")

	// Verify git config user.email
	cmd := exec.Command("git", "config", "user.email")
	cmd.Dir = workspace
	out, err := cmd.Output()
	assert.NoError(t, err)
	assert.Equal(t, "agent@recac.com", strings.TrimSpace(string(out)))

	// Verify we can commit
	dummyFile := filepath.Join(workspace, "test.txt")
	err = os.WriteFile(dummyFile, []byte("test"), 0644)
	assert.NoError(t, err)

	_, err = gitClient.Run(workspace, "add", "test.txt")
	assert.NoError(t, err)

	_, err = gitClient.Run(workspace, "commit", "-m", "Initial commit")
	assert.NoError(t, err)
}
