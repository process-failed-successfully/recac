package main

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"recac/internal/agent"
)

// MockBranchAgent for testing the branch command
type MockBranchAgent struct {
	mock.Mock
}

func (m *MockBranchAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockBranchAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	return "", nil
}

func (m *MockBranchAgent) Close() error {
	return nil
}

func TestBranchCmd(t *testing.T) {
	// Setup git mock
	originalGitFactory := gitClientFactory
	defer func() { gitClientFactory = originalGitFactory }()

	mockGit := &MockGitClient{
		RepoExistsFunc: func(repoPath string) bool { return true },
		RunFunc: func(repoPath string, args ...string) (string, error) {
			if len(args) == 3 && args[0] == "checkout" && args[1] == "-b" && args[2] == "jira-123-add-login-page" {
				return "Switched to a new branch 'jira-123-add-login-page'", nil
			}
			if len(args) == 3 && args[0] == "checkout" && args[1] == "-b" && args[2] == "fix-payment-bug" {
				return "Switched to a new branch 'fix-payment-bug'", nil
			}
			return "", nil
		},
	}
	gitClientFactory = func() IGitClient { return mockGit }

	// Setup agent mock
	originalAgentFactory := agentClientFactory
	defer func() { agentClientFactory = originalAgentFactory }()

	mockAgent := new(MockBranchAgent)

	// Expect it to be called with a specific prompt containing the issue key
	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "JIRA-123") && strings.Contains(prompt, "add login page")
	})).Return("jira-123-add-login-page", nil)

	// Expect it to be called with a specific prompt without issue key
	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return !strings.Contains(prompt, "JIRA-123") && strings.Contains(prompt, "fix payment bug")
	})).Return("fix-payment-bug", nil)

	agentClientFactory = func(ctx context.Context, provider, model, cwd, persona string) (agent.Agent, error) {
		return mockAgent, nil
	}

	t.Run("creates and checks out branch with issue key", func(t *testing.T) {
		cmd := NewBranchCmd()
		cmd.SetArgs([]string{"--issue-key", "JIRA-123", "add", "login", "page"})

		outBuf := new(bytes.Buffer)
		errBuf := new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)

		assert.Contains(t, errBuf.String(), "Generating branch name...")
		assert.Contains(t, errBuf.String(), "Checking out new branch 'jira-123-add-login-page'...")
		assert.Contains(t, outBuf.String(), "Successfully created and checked out branch: jira-123-add-login-page")
	})

	t.Run("dry-run does not checkout branch", func(t *testing.T) {
		cmd := NewBranchCmd()
		cmd.SetArgs([]string{"--dry-run", "fix payment bug"})

		outBuf := new(bytes.Buffer)
		errBuf := new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(errBuf)

		err := cmd.Execute()
		assert.NoError(t, err)

		assert.Contains(t, errBuf.String(), "Generating branch name...")
		assert.NotContains(t, errBuf.String(), "Checking out new branch") // Should not checkout
		assert.Contains(t, outBuf.String(), "Generated Branch Name (Dry Run): fix-payment-bug")
	})

	t.Run("fails if not a git repo", func(t *testing.T) {
		mockGitNoRepo := &MockGitClient{
			RepoExistsFunc: func(repoPath string) bool { return false },
		}

		oldFactory := gitClientFactory
		gitClientFactory = func() IGitClient { return mockGitNoRepo }
		defer func() { gitClientFactory = oldFactory }()

		cmd := NewBranchCmd()
		cmd.SetArgs([]string{"add", "login", "page"})

		err := cmd.Execute()
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "not a git repository")
	})
}
