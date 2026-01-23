package main

import (
	"context"
	"fmt"
	"testing"

	"recac/internal/agent"
	"recac/internal/git"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// BisectMockAgent for testing AI interaction
type BisectMockAgent struct {
	mock.Mock
}

func (m *BisectMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *BisectMockAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func TestBisectCmd(t *testing.T) {
	// Reset flags and global variables
	defer func() {
		gitClientFactory = func() IGitClient { return git.NewClient() }
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return nil, fmt.Errorf("mock agent factory not initialized")
		}
		bisectMaxSteps = 100
	}()

	t.Run("Success Flow with Convergence", func(t *testing.T) {
		mockGit := &MockGitClient{
			RepoExistsFunc: func(path string) bool { return true },
			BisectStartFunc: func(path, bad, good string) error {
				return nil
			},
			BisectResetFunc: func(path string) error { return nil },
		}

		commits := []string{"bad-commit", "halfway-commit", "good-commit"}
		idx := 0

		mockGit.CurrentCommitSHAFunc = func(path string) (string, error) {
			if idx < len(commits) {
				return commits[idx], nil
			}
			return commits[len(commits)-1], nil
		}

		mockGit.BisectGoodFunc = func(path, rev string) error {
			idx++ // Simulate moving to next commit
			return nil
		}
		mockGit.BisectBadFunc = func(path, rev string) error {
			idx++ // Simulate moving to next commit
			return nil
		}

		gitClientFactory = func() IGitClient { return mockGit }

		// Mock the command execution to simulate passing/failing tests
		// We can't easily mock executeShellCommand directly since it's a function in main.
		// However, we can use the "check command" flag to run a simple echo command
		// and interpret it? No, executeShellCommand runs real shell.
		// So we use a command that works cross-platform, like "echo" or "exit 0".
		// But verify logic depends on exit code.
		// "true" (exit 0) or "false" (exit 1).

		// Let's use "true" (or exit 0).
		// But we want to simulate failure too.
		// Since we cannot mock executeShellCommand, we rely on the command string itself.
		// We can pass `exit 0` for success and `exit 1` for failure.
		// On Windows `cmd /C exit 0` works. On sh `exit 0` works.

		// To simulate changing behavior based on commit, we can't easily do it with real shell commands
		// unless we write a file and check it.
		// But for this test, we just want to verify the bisect logic loops.

		// Actually, if we use a command that always succeeds (exit 0),
		// then IsGood = true -> BisectGood called -> idx increments -> loop.
		// Eventually idx reaches end -> CurrentCommitSHA stays same -> Convergence -> Success.

		output, err := executeCommand(rootCmd, "bisect", "--good", "v1", "--bad", "HEAD", "--command", "echo test")
		assert.NoError(t, err)
		assert.Contains(t, output, "Starting bisect")
		assert.Contains(t, output, "Result: GOOD") // Since echo exits 0
		assert.Contains(t, output, "Bisect converged")
	})

	t.Run("Infinite Loop Detection", func(t *testing.T) {
		bisectMaxSteps = 10
		mockGit := &MockGitClient{
			RepoExistsFunc:  func(path string) bool { return true },
			BisectStartFunc: func(path, bad, good string) error { return nil },
			BisectResetFunc: func(path string) error { return nil },
			CurrentCommitSHAFunc: func(path string) (string, error) {
				return "stuck-commit", nil
			},
			BisectGoodFunc: func(path, rev string) error { return nil }, // Doesn't move
		}
		gitClientFactory = func() IGitClient { return mockGit }

		// Use a command that returns success so we call BisectGood
		_, err := executeCommand(rootCmd, "bisect", "--good", "v1", "--command", "echo test")

		// Should succeed with "Converged" because CurrentSHA didn't change
		assert.NoError(t, err)
		// Wait, if SHA doesn't change, we break loop and return nil (success) printing "Converged".
		// Infinite loop is when we visit same SHA *after moving*?
		// My implementation:
		// 1. Get SHA. Check visited.
		// 2. Run command.
		// 3. Bisect Good/Bad.
		// 4. Check if new SHA == old SHA. If so, Converged.

		// So if BisectGood doesn't change SHA, we detect convergence immediately.
		// The "visited" check handles the case where we cycle: A -> B -> A.

		// Let's make it cycle.
		commits := []string{"commit-A", "commit-B", "commit-A"}
		idx := 0
		mockGit.CurrentCommitSHAFunc = func(path string) (string, error) {
			val := commits[idx%len(commits)]
			return val, nil
		}
		mockGit.BisectGoodFunc = func(path, rev string) error {
			idx++
			return nil
		}

		_, err = executeCommand(rootCmd, "bisect", "--good", "v1", "--command", "echo test")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "infinite loop detected")
	})

	t.Run("AI Verification", func(t *testing.T) {
		mockGit := &MockGitClient{
			RepoExistsFunc:  func(path string) bool { return true },
			BisectStartFunc: func(path, bad, good string) error { return nil },
			BisectResetFunc: func(path string) error { return nil },
			CurrentCommitSHAFunc: func(path string) (string, error) {
				return "commit-1", nil
			},
			BisectGoodFunc: func(path, rev string) error { return nil }, // Converge immediately
		}
		gitClientFactory = func() IGitClient { return mockGit }

		mockAgent := new(BisectMockAgent)
		mockAgent.On("Send", mock.Anything, mock.Anything).Return("The output looks GOOD.", nil)

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockAgent, nil
		}

		viper.Set("provider", "mock") // Ensure provider is set

		output, err := executeCommand(rootCmd, "bisect", "--good", "v1", "--command", "echo fail", "--ai-check")
		assert.NoError(t, err)
		assert.Contains(t, output, "Result: GOOD") // AI said GOOD despite echo output

		mockAgent.AssertExpectations(t)
	})
}
