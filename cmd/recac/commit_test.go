package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentCommit for testing
type MockAgentCommit struct {
	mock.Mock
}

func (m *MockAgentCommit) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentCommit) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

// MockGitClientCommit for testing
type MockGitClientCommit struct {
	mock.Mock
}

func (m *MockGitClientCommit) DiffStaged(dir string) (string, error) {
	args := m.Called(dir)
	return args.String(0), args.Error(1)
}

func (m *MockGitClientCommit) RepoExists(dir string) bool {
	args := m.Called(dir)
	return args.Bool(0)
}

func (m *MockGitClientCommit) Commit(dir, message string) error {
	args := m.Called(dir, message)
	return args.Error(0)
}

// Stub other methods
func (m *MockGitClientCommit) Clone(ctx context.Context, repoURL, directory string) error { return nil }
func (m *MockGitClientCommit) Config(directory, key, value string) error                  { return nil }
func (m *MockGitClientCommit) ConfigAddGlobal(key, value string) error                    { return nil }
func (m *MockGitClientCommit) RemoteBranchExists(directory, remote, branch string) (bool, error) {
	return false, nil
}
func (m *MockGitClientCommit) Fetch(directory, remote, branch string) error              { return nil }
func (m *MockGitClientCommit) Checkout(directory, branch string) error                   { return nil }
func (m *MockGitClientCommit) Push(directory, branch string) error                       { return nil }
func (m *MockGitClientCommit) Pull(directory, remote, branch string) error               { return nil }
func (m *MockGitClientCommit) Stash(directory string) error                              { return nil }
func (m *MockGitClientCommit) Merge(directory, branchName string) error                  { return nil }
func (m *MockGitClientCommit) AbortMerge(directory string) error                         { return nil }
func (m *MockGitClientCommit) Recover(directory string) error                            { return nil }
func (m *MockGitClientCommit) Clean(directory string) error                              { return nil }
func (m *MockGitClientCommit) ResetHard(directory, remote, branch string) error          { return nil }
func (m *MockGitClientCommit) StashPop(directory string) error                           { return nil }
func (m *MockGitClientCommit) DeleteRemoteBranch(directory, remote, branch string) error { return nil }
func (m *MockGitClientCommit) Diff(directory, startCommit, endCommit string) (string, error) {
	return "", nil
}
func (m *MockGitClientCommit) DiffStat(directory, startCommit, endCommit string) (string, error) {
	return "", nil
}
func (m *MockGitClientCommit) CurrentCommitSHA(directory string) (string, error) { return "", nil }
func (m *MockGitClientCommit) SetRemoteURL(directory, name, url string) error    { return nil }
func (m *MockGitClientCommit) DeleteLocalBranch(directory, branch string) error  { return nil }
func (m *MockGitClientCommit) LocalBranchExists(directory, branch string) (bool, error) {
	return false, nil
}
func (m *MockGitClientCommit) Log(directory string, args ...string) ([]string, error) {
	return []string{}, nil
}
func (m *MockGitClientCommit) CurrentBranch(directory string) (string, error)   { return "main", nil }
func (m *MockGitClientCommit) CheckoutNewBranch(directory, branch string) error { return nil }

func (m *MockGitClientCommit) BisectStart(directory, bad, good string) error { return nil }
func (m *MockGitClientCommit) BisectGood(directory, rev string) error        { return nil }
func (m *MockGitClientCommit) BisectBad(directory, rev string) error         { return nil }
func (m *MockGitClientCommit) BisectReset(directory string) error            { return nil }
func (m *MockGitClientCommit) BisectLog(directory string) ([]string, error)  { return []string{}, nil }

func TestCommitCmd(t *testing.T) {
	// Setup mocks
	origGitFactory := gitClientFactory
	origAgentFactory := agentClientFactory
	defer func() {
		gitClientFactory = origGitFactory
		agentClientFactory = origAgentFactory
	}()

	mockGit := new(MockGitClientCommit)
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	mockAgent := new(MockAgentCommit)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Change to temp dir to satisfy os.Getwd() calls if needed, though logic uses returned path
	tempDir, _ := os.MkdirTemp("", "recac-commit-test")
	defer os.RemoveAll(tempDir)
	cwd, _ := os.Getwd()
	// Tests run in repo root usually, but command uses os.Getwd()
	// We'll just expect mock calls with whatever os.Getwd() returns.
	// Actually, better to chdir to tempDir so we know the path.
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	t.Run("No Staged Changes", func(t *testing.T) {
		mockGit.On("RepoExists", tempDir).Return(true).Once()
		mockGit.On("DiffStaged", tempDir).Return("", nil).Once()

		cmd := NewCommitCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no staged changes found")
		mockGit.AssertExpectations(t)
	})

	t.Run("Generate Message Only (Dry Run)", func(t *testing.T) {
		diff := "diff --git a/foo b/foo\n..."
		mockGit.On("RepoExists", tempDir).Return(true).Once()
		mockGit.On("DiffStaged", tempDir).Return(diff, nil).Once()

		generatedMsg := "feat: add foo"
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(generatedMsg, nil).Once()

		cmd := NewCommitCmd()
		outBuf := new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, outBuf.String(), generatedMsg)
		assert.Contains(t, outBuf.String(), "Tip: Run with --yes")

		mockGit.AssertNotCalled(t, "Commit")
		mockGit.AssertExpectations(t)
		mockAgent.AssertExpectations(t)
	})

	t.Run("Generate and Commit (--yes)", func(t *testing.T) {
		diff := "diff --git a/bar b/bar\n..."
		mockGit.On("RepoExists", tempDir).Return(true).Once()
		mockGit.On("DiffStaged", tempDir).Return(diff, nil).Once()

		generatedMsg := "fix: bar bug"
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(generatedMsg, nil).Once()
		mockGit.On("Commit", tempDir, generatedMsg).Return(nil).Once()

		cmd := NewCommitCmd()
		outBuf := new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--yes"})

		err := cmd.Execute()
		assert.NoError(t, err)
		assert.Contains(t, outBuf.String(), generatedMsg)
		assert.Contains(t, outBuf.String(), "Changes committed successfully")

		mockGit.AssertExpectations(t)
		mockAgent.AssertExpectations(t)
	})

	t.Run("Agent Error", func(t *testing.T) {
		diff := "diff..."
		mockGit.On("RepoExists", tempDir).Return(true).Once()
		mockGit.On("DiffStaged", tempDir).Return(diff, nil).Once()

		mockAgent.On("Send", mock.Anything, mock.Anything).Return("", fmt.Errorf("agent error")).Once()

		cmd := NewCommitCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate commit message")
	})

	t.Run("Not A Git Repo", func(t *testing.T) {
		mockGit.On("RepoExists", tempDir).Return(false).Once()

		cmd := NewCommitCmd()
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a git repository")
	})
}
