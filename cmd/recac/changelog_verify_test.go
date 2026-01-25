package main

import (
	"context"
	"os"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestChangelogVerifyCmd(t *testing.T) {
	// Setup Factories
	origGitFactory := gitClientFactory
	origAgentFactory := agentClientFactory
	defer func() {
		gitClientFactory = origGitFactory
		agentClientFactory = origAgentFactory
	}()

	mockGit := &MockGitClient{}
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	mockAgent := new(MockAgentCommit)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Setup Temp Dir
	tempDir, _ := os.MkdirTemp("", "recac-changelog-verify-test")
	defer os.RemoveAll(tempDir)
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	// Create dummy changelog
	os.WriteFile("CHANGELOG.md", []byte("Initial content"), 0644)

	t.Run("No Changes", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.RunFunc = func(dir string, args ...string) (string, error) {
			assert.Equal(t, "diff", args[0])
			return "", nil // No changes
		}

		output, err := executeCommand(rootCmd, "changelog", "verify", "--base", "main")

		// executeCommand suppresses exit(1) but returns output.
		// If command returns error, executeCommand returns error.
		// Wait, executeCommand catches panic("exit-1") but err might be nil if runE returns err?
		// My executeCommand implementation:
		// err = root.Execute() -> returns error from RunE.
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no changes found")
		_ = output
	})

	t.Run("Changes Detected - No Strict", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.RunFunc = func(dir string, args ...string) (string, error) {
			return "diff content", nil
		}

		output, err := executeCommand(rootCmd, "changelog", "verify", "--base", "main")

		assert.NoError(t, err)
		assert.Contains(t, output, "Changes detected")
	})

	t.Run("Strict Mode - PASS", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.RunFunc = func(dir string, args ...string) (string, error) {
			if args[0] == "diff" {
				return "+ new feature", nil
			}
			return "", nil
		}
		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{"hash feat: something"}, nil
		}

		mockAgent.ExpectedCalls = nil
		mockAgent.On("Send", mock.Anything, mock.Anything).Return("PASS", nil).Once()

		output, err := executeCommand(rootCmd, "changelog", "verify", "--strict", "--base", "main")

		assert.NoError(t, err)
		assert.Contains(t, output, "AI Verification passed")

		mockAgent.AssertExpectations(t)
	})

	t.Run("Strict Mode - FAIL", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.RunFunc = func(dir string, args ...string) (string, error) {
			if args[0] == "diff" {
				return "+ trivial change", nil
			}
			return "", nil
		}
		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{"hash feat: big feature"}, nil
		}

		mockAgent.ExpectedCalls = nil
		mockAgent.On("Send", mock.Anything, mock.Anything).Return("FAIL: Missing feature", nil).Once()

		output, err := executeCommand(rootCmd, "changelog", "verify", "--strict", "--base", "main")

		assert.Error(t, err)
		assert.Contains(t, err.Error(), "verification failed")

		// The error details might be in output or err.Error depending on implementation.
		// My implementation: fmt.Fprintf(cmd.ErrOrStderr(), "❌ Verification failed:\n%s\n", resp)
		// executeCommand captures ErrOrStderr.
		assert.Contains(t, output, "Verification failed")

		mockAgent.AssertExpectations(t)
	})
}
