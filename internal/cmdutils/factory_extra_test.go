package cmdutils

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockGitClientForCleanup is a separate mock implementation to avoid conflicts
// and allow specific behaviors for the cleanup test.
type MockGitClientForCleanup struct {
	MockGitClient
}

func TestSetupWorkspace_CleansNonGitDirectory(t *testing.T) {
	// Setup temporary directory
	tmpDir, err := os.MkdirTemp("", "recac-test-workspace")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create some junk files in it
	err = os.WriteFile(filepath.Join(tmpDir, "junk.txt"), []byte("junk"), 0644)
	assert.NoError(t, err)
	err = os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "subdir", "junk2.txt"), []byte("junk2"), 0644)
	assert.NoError(t, err)

	// Mock Git Client
	mockGit := &MockGitClientForCleanup{}

	// Override RepoExists to return false (triggering cleanup logic)
	mockGit.repoExists = false

	// Override Clone to simulate successful clone by creating .git directory
	mockGit.cloneFn = func(ctx context.Context, repoURL, directory string) error {
		return os.MkdirAll(filepath.Join(directory, ".git"), 0755)
	}

	ctx := context.Background()
	repoURL := "https://github.com/example/repo.git"
	ticketID := "TEST-123"

	// Run SetupWorkspace
	_, err = SetupWorkspace(ctx, mockGit, repoURL, tmpDir, ticketID, "", "")
	assert.NoError(t, err)

	// Verify that the directory was cleaned (junk files gone)
	_, err = os.Stat(filepath.Join(tmpDir, "junk.txt"))
	assert.True(t, os.IsNotExist(err), "junk.txt should have been deleted")

	_, err = os.Stat(filepath.Join(tmpDir, "subdir"))
	assert.True(t, os.IsNotExist(err), "subdir should have been deleted")

	// Verify .git exists (created by mock clone)
	_, err = os.Stat(filepath.Join(tmpDir, ".git"))
	assert.NoError(t, err, ".git directory should exist")
}
