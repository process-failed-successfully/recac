package main

import (
	"context"
	"os/exec"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPRCmd(t *testing.T) {
	// Setup Mocks
	mockGit := &MockGitClient{}
	mockAgent := new(MockAgent)

	// Override Factories
	oldGitFactory := gitClientFactory
	oldAgentFactory := agentClientFactory
	oldExecCommand := execCommand
	defer func() {
		gitClientFactory = oldGitFactory
		agentClientFactory = oldAgentFactory
		execCommand = oldExecCommand
	}()

	gitClientFactory = func() IGitClient {
		return mockGit
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	t.Run("Generate Only (Default)", func(t *testing.T) {
		// Mock Git
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.CurrentBranchFunc = func(repoPath string) (string, error) { return "feature/foo", nil }
		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{"hash1 Author: feat: foo"}, nil
		}
		mockGit.DiffFunc = func(repoPath, commitA, commitB string) (string, error) {
			return "diff content", nil
		}

		// Mock Agent
		expectedResponse := "TITLE: feat: foo\nBODY:\nImplemented foo"
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(expectedResponse, nil).Once()

		// Execute
		cmd, _, _ := newRootCmd()
		// prCmd is added in init(), so it should be there.
		// But newRootCmd might create a fresh one?
		// Actually newRootCmd in `main_test.go` (if it exists) or similar.
		// Usually tests in `main` package just use the global `rootCmd` or create a new one and add commands.
		// Let's check `test_helpers_test.go` to see how `newRootCmd` is implemented or if we should use `rootCmd`.
		// Since I can't read it right now, I'll rely on what I saw in `changelog_test.go`:
		// `cmd, _, _ := newRootCmd(); cmd.AddCommand(changelogCmd)`

		// I'll create a new root for isolation
		cmd = &cobra.Command{}
		cmd.AddCommand(prCmd)

		// Reset flags
		prCreate = false
		prDraft = false
		prTitleOnly = false
		prBodyOnly = false
		prBase = "main" // Reset default

		output, err := executeCommand(cmd, "pr")
		assert.NoError(t, err)
		assert.Contains(t, output, "Title: feat: foo")
		assert.Contains(t, output, "Implemented foo")
	})

	t.Run("Create with gh", func(t *testing.T) {
		// Mock Git
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.CurrentBranchFunc = func(repoPath string) (string, error) { return "feature/bar", nil }
		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{"hash1: fix bar"}, nil
		}
		mockGit.DiffFunc = func(repoPath, commitA, commitB string) (string, error) {
			return "diff bar", nil
		}

		// Mock Agent
		expectedResponse := "TITLE: fix: bar\nBODY:\nFixed bar"
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(expectedResponse, nil).Once()

		// Mock exec.Command for gh
		// We use the helper pattern: execCommand calls a helper process.
		// But `execCommand` is a variable we can swap.
		// We can swap it to return a *exec.Cmd that calls "echo" or similar.

		var capturedArgs []string
		execCommand = func(name string, args ...string) *exec.Cmd {
			capturedArgs = append([]string{name}, args...)
			// Return a dummy command that succeeds
			return exec.Command("true")
		}

		cmd := &cobra.Command{}
		cmd.AddCommand(prCmd)

		// Flags need to be passed in args or set directly if using executeCommand helper which parses args?
		// executeCommand parses args.
		output, err := executeCommand(cmd, "pr", "--create", "--draft")
		assert.NoError(t, err)
		assert.Contains(t, output, "Creating PR on main...")

		// Verify gh was called with correct args
		assert.Equal(t, "gh", capturedArgs[0])
		assert.Contains(t, capturedArgs, "pr")
		assert.Contains(t, capturedArgs, "create")
		assert.Contains(t, capturedArgs, "--draft")
		assert.Contains(t, capturedArgs, "fix: bar")
	})

	t.Run("No Commits", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.CurrentBranchFunc = func(repoPath string) (string, error) { return "feature/empty", nil }
		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{}, nil
		}

		cmd := &cobra.Command{}
		cmd.AddCommand(prCmd)

		_, err := executeCommand(cmd, "pr")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no commits found")
	})

	t.Run("Already on Base", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }
		mockGit.CurrentBranchFunc = func(repoPath string) (string, error) { return "main", nil }

		cmd := &cobra.Command{}
		cmd.AddCommand(prCmd)

		_, err := executeCommand(cmd, "pr")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "already on the base branch")
	})
}
