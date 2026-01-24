package main

import (
	"bytes"
	"context"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReleaseCmd(t *testing.T) {
	// Setup Mocks
	mockGit := &MockGitClient{}

	// Override Git Factory
	originalGitFactory := gitClientFactory
	gitClientFactory = func() IGitClient {
		return mockGit
	}
	defer func() { gitClientFactory = originalGitFactory }()

	// Override Agent Factory
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		mockAgent := agent.NewMockAgent()
		// Mock response: Version on first line, Changelog on rest
		mockAgent.SetResponse("v1.1.0\n## v1.1.0\n- Feat: Cool stuff")
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// We don't need to chdir, as we can't easily mock os.Getwd() for the command execution,
	// but the command uses gitClient which we mocked, so the path passed to gitClient methods will be the real CWD.
	// Our mock checks args, but we can relax checks or just ignore path arg.

	t.Run("Full Flow with Push", func(t *testing.T) {
		// Expectations
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.FetchFunc = func(path, remote, tag string) error { return nil }
		mockGit.LatestTagFunc = func(path string) (string, error) { return "v1.0.0", nil }
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{"abc1234 feat: cool stuff"}, nil
		}

		tagCalled := false
		mockGit.TagFunc = func(path, version string) error {
			if version == "v1.1.0" {
				tagCalled = true
			}
			return nil
		}

		pushCalled := false
		mockGit.PushTagsFunc = func(path string) error {
			pushCalled = true
			return nil
		}

		output, err := executeCommand(rootCmd, "release", "--yes", "--push")
		require.NoError(t, err)

		assert.True(t, tagCalled, "Tag should be called")
		assert.True(t, pushCalled, "PushTags should be called")
		assert.Contains(t, output, "Version: v1.0.0 -> v1.1.0")
		assert.Contains(t, output, "Released v1.1.0 successfully")
	})

	t.Run("No changes", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.FetchFunc = func(path, remote, tag string) error { return nil }
		mockGit.LatestTagFunc = func(path string) (string, error) { return "v1.0.0", nil }
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{}, nil // Empty logs
		}

		output, err := executeCommand(rootCmd, "release")
		require.NoError(t, err)

		assert.Contains(t, output, "No new commits")
	})

	t.Run("First Release", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.FetchFunc = func(path, remote, tag string) error { return nil }
		mockGit.LatestTagFunc = func(path string) (string, error) { return "", nil } // No tags
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{"initial commit"}, nil
		}

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			mockAgent := agent.NewMockAgent()
			mockAgent.SetResponse("v0.1.0\n## v0.1.0\n- Initial")
			return mockAgent, nil
		}

		tagCalled := false
		mockGit.TagFunc = func(path, version string) error {
			if version == "v0.1.0" {
				tagCalled = true
			}
			return nil
		}

		output, err := executeCommand(rootCmd, "release", "--yes")
		require.NoError(t, err)

		assert.Contains(t, output, "Assuming v0.0.0 start")
		assert.Contains(t, output, "Version: v0.0.0 -> v0.1.0")
		assert.True(t, tagCalled)
	})

	t.Run("Interactive Abort", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.FetchFunc = func(path, remote, tag string) error { return nil }
		mockGit.LatestTagFunc = func(path string) (string, error) { return "v1.0.0", nil }
		mockGit.LogFunc = func(path string, args ...string) ([]string, error) {
			return []string{"fix: bug"}, nil
		}

		// Revert factory to default mock response just in case
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			mockAgent := agent.NewMockAgent()
			mockAgent.SetResponse("v1.0.1\n## v1.0.1\n- Fix: Bug")
			return mockAgent, nil
		}

		// Setup stdin with "n"
		var inBuf bytes.Buffer
		inBuf.WriteString("n\n")
		rootCmd.SetIn(&inBuf)

		// Do not use --yes, so it prompts
		// executeCommand uses resetFlags which clears --yes if set previously
		output, err := executeCommand(rootCmd, "release")
		require.NoError(t, err)

		assert.Contains(t, output, "Aborted")
	})
}
