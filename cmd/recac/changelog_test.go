package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestChangelogCmd(t *testing.T) {
	// Setup Mocks
	mockGit := &MockGitClient{}
	mockAgent := new(MockAgent) // Reuse MockAgent from tickets_test.go

	// Override Factories
	oldGitFactory := gitClientFactory
	oldAgentFactory := agentClientFactory
	defer func() {
		gitClientFactory = oldGitFactory
		agentClientFactory = oldAgentFactory
	}()

	gitClientFactory = func() IGitClient {
		return mockGit
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	t.Run("Success", func(t *testing.T) {
		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{
				"hash1 Author1: feat: added login",
				"hash2 Author2: fix: typo in readme",
			}, nil
		}
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }

		expectedChangelog := "# Changelog\n\n## Features\n- feat: added login\n\n## Fixes\n- fix: typo in readme"
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(expectedChangelog, nil).Once()

		cmd, _, _ := newRootCmd()
		// We need to ensure we are calling the changelog subcommand
		cmd.AddCommand(changelogCmd)
		// Note: root.go likely adds it in init(), but when testing, we rely on executeCommand calling root.Execute()
		// verify if init() runs. Yes.

		// However, executeCommand resets flags.
		// Let's use executeCommand helper if possible.

		output, err := executeCommand(cmd, "changelog")
		assert.NoError(t, err)
		assert.Contains(t, output, "# Changelog")
		assert.Contains(t, output, "feat: added login")
	})

	t.Run("No Commits", func(t *testing.T) {
		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{}, nil
		}
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }

		_, err := executeCommand(rootCmd, "changelog")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no commits found")
	})

	t.Run("Output to File", func(t *testing.T) {
		mockGit.LogFunc = func(repoPath string, args ...string) ([]string, error) {
			return []string{"hash1 Author: msg"}, nil
		}
		mockGit.RepoExistsFunc = func(repoPath string) bool { return true }

		expectedChangelog := "# Changelog"
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(expectedChangelog, nil).Once()

		tmpDir, err := os.MkdirTemp("", "changelog-test")
		assert.NoError(t, err)
		defer os.RemoveAll(tmpDir)

		outFile := filepath.Join(tmpDir, "CHANGELOG.md")

		_, err = executeCommand(rootCmd, "changelog", "--output", outFile)
		assert.NoError(t, err)

		content, err := os.ReadFile(outFile)
		assert.NoError(t, err)
		assert.Equal(t, expectedChangelog, string(content))
	})
}
