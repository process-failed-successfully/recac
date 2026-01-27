package main

import (
	"context"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentForPr is a mock implementation of the Agent interface for PR tests.
type MockAgentForPr struct {
	mock.Mock
}

func (m *MockAgentForPr) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentForPr) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	return args.String(0), args.Error(1)
}

func TestPRCmd(t *testing.T) {
	// 1. Setup
	mockGit := &MockGitClient{}
	mockAgent := new(MockAgentForPr)

	// Override factories
	originalGitFactory := gitClientFactory
	originalAgentFactory := agentClientFactory
	defer func() {
		gitClientFactory = originalGitFactory
		agentClientFactory = originalAgentFactory
	}()

	gitClientFactory = func() IGitClient {
		return mockGit
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	t.Run("Generate PR Description", func(t *testing.T) {
		root, _, _ := newRootCmd()

		// Mock git calls
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.CurrentBranchFunc = func(repoPath string) (string, error) { return "feature/new-feature", nil }
		mockGit.DiffFunc = func(repoPath, commitA, commitB string) (string, error) {
			assert.Equal(t, "main", commitA)
			assert.Equal(t, "feature/new-feature", commitB)
			return "diff content", nil
		}

		// Mock Agent call
		mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
			return true
		})).Return(`{"title": "New Feature", "description": "This adds a new feature"}`, nil)

		// Run
		output, err := executeCommand(root, "pr", "--dry-run")
		assert.NoError(t, err)

		// Assertions
		assert.Contains(t, output, "New Feature")
		assert.Contains(t, output, "This adds a new feature")

		mockAgent.AssertExpectations(t)
	})

	t.Run("Create PR", func(t *testing.T) {
		root, _, _ := newRootCmd()

		// Mock git calls
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.CurrentBranchFunc = func(repoPath string) (string, error) { return "feature/create", nil }
		mockGit.DiffFunc = func(repoPath, commitA, commitB string) (string, error) {
			return "diff content", nil
		}
		mockGit.CreatePRFunc = func(repoPath, title, body, base string) (string, error) {
			assert.Equal(t, "Auto PR", title)
			return "http://github.com/pr/1", nil
		}

		// Mock Agent
		// Use a different mock agent to reset expectations or just reuse
		// Since we mocked the factory, we should probably reset mockAgent or create new one.
		// But newRootCmd is called, factories are global.
		// Let's reset mockAgent expectations.
		mockAgent.ExpectedCalls = nil
		mockAgent.Calls = nil

		mockAgent.On("Send", mock.Anything, mock.Anything).Return(`{"title": "Auto PR", "description": "Desc"}`, nil)

		// Run
		output, err := executeCommand(root, "pr", "--create")
		assert.NoError(t, err)

		// Assertions
		assert.Contains(t, output, "PR Created: http://github.com/pr/1")
	})

	t.Run("Empty Diff", func(t *testing.T) {
		root, _, _ := newRootCmd()

		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.CurrentBranchFunc = func(repoPath string) (string, error) { return "main", nil }
		mockGit.DiffFunc = func(repoPath, commitA, commitB string) (string, error) {
			return "", nil
		}

		_, err := executeCommand(root, "pr")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no changes detected")
	})
}
