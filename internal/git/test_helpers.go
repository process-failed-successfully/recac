package git

import (
	"context"
	"github.com/stretchr/testify/mock"
)

// MockGitClient is a mock of the git.Client for testing purposes.
type MockGitClient struct {
	mock.Mock
	repoExists         bool
	remoteBranchExists bool
	cloneFn            func(ctx context.Context, repoURL, directory string) error
	checkoutFn         func(directory, branch string) error
	checkoutNewBranchFn func(directory, branch string) error
	shortStatus        string
	shortStatusErr     error
}

func (m *MockGitClient) DiffStat(workspace, startCommit, endCommit string) (string, error) {
	args := m.Called(workspace, startCommit, endCommit)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) CurrentCommitSHA(workspace string) (string, error) {
	args := m.Called(workspace)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) Clone(ctx context.Context, repoURL, directory string) error {
	if m.cloneFn != nil {
		return m.cloneFn(ctx, repoURL, directory)
	}
	args := m.Called(ctx, repoURL, directory)
	return args.Error(0)
}

func (m *MockGitClient) RepoExists(directory string) bool {
	if m.repoExists {
		return m.repoExists
	}
	args := m.Called(directory)
	return args.Bool(0)
}

func (m *MockGitClient) Config(directory, key, value string) error {
	args := m.Called(directory, key, value)
	return args.Error(0)
}

func (m *MockGitClient) ConfigAddGlobal(key, value string) error {
	args := m.Called(key, value)
	return args.Error(0)
}

func (m *MockGitClient) RemoteBranchExists(directory, remote, branch string) (bool, error) {
	if m.remoteBranchExists {
		return m.remoteBranchExists, nil
	}
	args := m.Called(directory, remote, branch)
	return args.Bool(0), args.Error(1)
}

func (m *MockGitClient) Fetch(directory, remote, branch string) error {
	args := m.Called(directory, remote, branch)
	return args.Error(0)
}

func (m *MockGitClient) Stash(directory string) error {
	args := m.Called(directory)
	return args.Error(0)
}

func (m *MockGitClient) Merge(directory, branchName string) error {
	args := m.Called(directory, branchName)
	return args.Error(0)
}

func (m *MockGitClient) AbortMerge(directory string) error {
	args := m.Called(directory)
	return args.Error(0)
}

func (m *MockGitClient) Recover(directory string) error {
	args := m.Called(directory)
	return args.Error(0)
}

func (m *MockGitClient) Clean(directory string) error {
	args := m.Called(directory)
	return args.Error(0)
}

func (m *MockGitClient) ResetHard(directory, remote, branch string) error {
	args := m.Called(directory, remote, branch)
	return args.Error(0)
}

func (m *MockGitClient) StashPop(directory string) error {
	args := m.Called(directory)
	return args.Error(0)
}

func (m *MockGitClient) DeleteRemoteBranch(directory, remote, branch string) error {
	args := m.Called(directory, remote, branch)
	return args.Error(0)
}

func (m *MockGitClient) CurrentBranch(directory string) (string, error) {
	args := m.Called(directory)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) Commit(directory, message string) error {
	args := m.Called(directory, message)
	return args.Error(0)
}

func (m *MockGitClient) Diff(directory, startCommit, endCommit string) (string, error) {
	args := m.Called(directory, startCommit, endCommit)
	return args.String(0), args.Error(1)
}

func (m *MockGitClient) SetRemoteURL(directory, name, url string) error {
	args := m.Called(directory, name, url)
	return args.Error(0)
}

func (m *MockGitClient) DeleteLocalBranch(directory, branch string) error {
	args := m.Called(directory, branch)
	return args.Error(0)
}

func (m *MockGitClient) LocalBranchExists(directory, branch string) (bool, error) {
	args := m.Called(directory, branch)
	return args.Bool(0), args.Error(1)
}

func (m *MockGitClient) Checkout(directory, branch string) error {
	if m.checkoutFn != nil {
		return m.checkoutFn(directory, branch)
	}
	args := m.Called(directory, branch)
	return args.Error(0)
}

func (m *MockGitClient) CheckoutNewBranch(directory, branch string) error {
	if m.checkoutNewBranchFn != nil {
		return m.checkoutNewBranchFn(directory, branch)
	}
	args := m.Called(directory, branch)
	return args.Error(0)
}

func (m *MockGitClient) Push(directory, branch string) error {
	args := m.Called(directory, branch)
	return args.Error(0)
}

func (m *MockGitClient) Pull(directory, remote, branch string) error {
	args := m.Called(directory, remote, branch)
	return args.Error(0)
}

func (m *MockGitClient) GetShortStatus(workspace string) (string, error) {
	if m.shortStatusErr != nil {
		return "", m.shortStatusErr
	}
	if m.shortStatus != "" {
		return m.shortStatus, nil
	}
	args := m.Called(workspace)
	return args.String(0), args.Error(1)
}
