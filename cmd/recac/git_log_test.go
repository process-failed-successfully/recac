package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGitClientLog for testing
type MockGitClientLog struct {
	mock.Mock
}

// Implement only used methods
func (m *MockGitClientLog) RepoExists(dir string) bool {
	args := m.Called(dir)
	return args.Bool(0)
}

func (m *MockGitClientLog) Log(directory string, args ...string) ([]string, error) {
	callArgs := m.Called(directory, args)
	return callArgs.Get(0).([]string), callArgs.Error(1)
}

func (m *MockGitClientLog) Diff(directory, startCommit, endCommit string) (string, error) {
	args := m.Called(directory, startCommit, endCommit)
	return args.String(0), args.Error(1)
}

// Stubs for interface satisfaction
func (m *MockGitClientLog) DiffStat(workspace, startCommit, endCommit string) (string, error) { return "", nil }
func (m *MockGitClientLog) CurrentCommitSHA(workspace string) (string, error) { return "", nil }
func (m *MockGitClientLog) Clone(ctx context.Context, repoURL, directory string) error { return nil }
func (m *MockGitClientLog) Config(directory, key, value string) error { return nil }
func (m *MockGitClientLog) ConfigGlobal(key, value string) error { return nil }
func (m *MockGitClientLog) ConfigAddGlobal(key, value string) error { return nil }
func (m *MockGitClientLog) RemoteBranchExists(directory, remote, branch string) (bool, error) { return false, nil }
func (m *MockGitClientLog) Fetch(directory, remote, branch string) error { return nil }
func (m *MockGitClientLog) Checkout(directory, branch string) error { return nil }
func (m *MockGitClientLog) CheckoutNewBranch(directory, branch string) error { return nil }
func (m *MockGitClientLog) Push(directory, branch string) error { return nil }
func (m *MockGitClientLog) Pull(directory, remote, branch string) error { return nil }
func (m *MockGitClientLog) Stash(directory string) error { return nil }
func (m *MockGitClientLog) Merge(directory, branchName string) error { return nil }
func (m *MockGitClientLog) AbortMerge(directory string) error { return nil }
func (m *MockGitClientLog) Recover(directory string) error { return nil }
func (m *MockGitClientLog) Clean(directory string) error { return nil }
func (m *MockGitClientLog) ResetHard(directory, remote, branch string) error { return nil }
func (m *MockGitClientLog) StashPop(directory string) error { return nil }
func (m *MockGitClientLog) DeleteRemoteBranch(directory, remote, branch string) error { return nil }
func (m *MockGitClientLog) CurrentBranch(directory string) (string, error) { return "", nil }
func (m *MockGitClientLog) Commit(directory, message string) error { return nil }
func (m *MockGitClientLog) DiffStaged(directory string) (string, error) { return "", nil }
func (m *MockGitClientLog) SetRemoteURL(directory, name, url string) error { return nil }
func (m *MockGitClientLog) DeleteLocalBranch(directory, branch string) error { return nil }
func (m *MockGitClientLog) LocalBranchExists(directory, branch string) (bool, error) { return false, nil }
func (m *MockGitClientLog) BisectStart(directory, bad, good string) error { return nil }
func (m *MockGitClientLog) BisectGood(directory, rev string) error { return nil }
func (m *MockGitClientLog) BisectBad(directory, rev string) error { return nil }
func (m *MockGitClientLog) BisectReset(directory string) error { return nil }
func (m *MockGitClientLog) BisectLog(directory string) ([]string, error) { return nil, nil }
func (m *MockGitClientLog) Tag(directory, version string) error { return nil }
func (m *MockGitClientLog) DeleteTag(directory, version string) error { return nil }
func (m *MockGitClientLog) PushTags(directory string) error { return nil }
func (m *MockGitClientLog) LatestTag(directory string) (string, error) { return "", nil }
func (m *MockGitClientLog) Run(directory string, args ...string) (string, error) { return "", nil }


func TestGitLogCmd(t *testing.T) {
	// Setup Mocks
	origGitFactory := gitClientFactory
	defer func() { gitClientFactory = origGitFactory }()

	mockGit := new(MockGitClientLog)
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	// Change to temp dir
	tempDir, _ := os.MkdirTemp("", "recac-git-log-test")
	defer os.RemoveAll(tempDir)
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	t.Run("Not a Git Repo", func(t *testing.T) {
		mockGit.On("RepoExists", mock.Anything).Return(false).Once()

		cmd := gitLogCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "not a git repository")
		}
		mockGit.AssertExpectations(t)
	})

	t.Run("Fetch Log Error", func(t *testing.T) {
		mockGit.On("RepoExists", mock.Anything).Return(true).Once()
		mockGit.On("Log", mock.Anything, mock.Anything).Return([]string{}, errors.New("git log failed")).Once()

		cmd := gitLogCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "failed to fetch git log")
		}
		mockGit.AssertExpectations(t)
	})

	t.Run("Empty Log", func(t *testing.T) {
		mockGit.On("RepoExists", mock.Anything).Return(true).Once()
		mockGit.On("Log", mock.Anything, mock.Anything).Return([]string{}, nil).Once()

		cmd := gitLogCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.RunE(cmd, []string{})
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "no commits found")
		}
		mockGit.AssertExpectations(t)
	})

	// We cannot easily test successful run because it launches TUI (tea.Program.Run)
	// which will panic or hang in test environment without TTY, or requires
	// tea.WithInput(nil) which we didn't expose option for.
	// But validating initialization and error handling gives good confidence.
}
