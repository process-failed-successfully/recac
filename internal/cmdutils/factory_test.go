package cmdutils

import (
	"context"
	"os"
	"recac/internal/git"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

type MockGitClient struct {
	git.Client
	CloneFunc func(ctx context.Context, url, dest string) error
	RunFunc   func(dir string, args ...string) (string, error)
}

func (m *MockGitClient) Clone(ctx context.Context, url, dest string) error {
	if m.CloneFunc != nil {
		return m.CloneFunc(ctx, url, dest)
	}
	return nil
}

func (m *MockGitClient) Run(dir string, args ...string) (string, error) {
	if m.RunFunc != nil {
		return m.RunFunc(dir, args...)
	}
	return "", nil
}

func (m *MockGitClient) RepoExists(dir string) bool { return false }
func (m *MockGitClient) Config(dir, key, value string) error { return nil }
func (m *MockGitClient) RemoteBranchExists(dir, remote, branch string) (bool, error) { return false, nil }
func (m *MockGitClient) ConfigAddGlobal(key, value string) error { return nil }
func (m *MockGitClient) CheckoutNewBranch(dir, branch string) error { return nil }
func (m *MockGitClient) Push(dir, branch string) error { return nil }
// Add missing interfaces
func (m *MockGitClient) CurrentCommitSHA(repoPath string) (string, error) { return "", nil }

func TestSetupWorkspace_MockProvider(t *testing.T) {
	viper.Set("provider", "mock")
	viper.Set("git.unique_branch_names", false)

	// Create temp dir for workspace
	tmpDir, err := os.MkdirTemp("", "ws-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	mockGit := &MockGitClient{
		CloneFunc: func(ctx context.Context, url, dest string) error {
			return assert.AnError // Simulate Clone failure
		},
		RunFunc: func(dir string, args ...string) (string, error) {
			if args[0] == "init" {
				return "", nil
			}
			if args[0] == "commit" {
				return "", nil
			}
			return "", nil
		},
	}

	repoURL, err := SetupWorkspace(context.Background(), mockGit, "http://bad.repo", tmpDir, "TICKET-1", "", "")

	assert.NoError(t, err)
	assert.Equal(t, "http://bad.repo", repoURL)
}
