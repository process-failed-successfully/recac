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

// MockAgentSquash for testing
type MockAgentSquash struct {
	mock.Mock
}

func (m *MockAgentSquash) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentSquash) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestSquashCmd(t *testing.T) {
	// Setup mocks
	origGitFactory := gitClientFactory
	origAgentFactory := agentClientFactory
	defer func() {
		gitClientFactory = origGitFactory
		agentClientFactory = origAgentFactory
	}()

	mockGit := new(MockGitClient)
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	mockAgent := new(MockAgentSquash)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	tempDir, _ := os.MkdirTemp("", "recac-squash-test")
	defer os.RemoveAll(tempDir)
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	t.Run("No Commits", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(dir string) bool { return true }
		mockGit.MergeBaseFunc = func(dir, ref1, ref2 string) (string, error) { return "abcdef", nil }
		mockGit.LogFunc = func(dir string, args ...string) ([]string, error) { return []string{}, nil }

		cmd := squashCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		// Important: Set args carefully so we don't carry over flags from other tests
		cmd.SetArgs([]string{})

		err := runSquash(cmd, []string{})
		assert.NoError(t, err)
	})

	t.Run("Generate Squash Message (Dry Run)", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(dir string) bool { return true }
		mockGit.MergeBaseFunc = func(dir, ref1, ref2 string) (string, error) { return "abcdef", nil }
		mockGit.LogFunc = func(dir string, args ...string) ([]string, error) {
			return []string{"12345: WIP", "67890: Fix typos"}, nil
		}
		mockGit.DiffFunc = func(dir, start, end string) (string, error) { return "diff --git a/foo b/foo", nil }

		generatedMsg := "feat: squash feature updates"
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(generatedMsg, nil).Once()

		cmd := squashCmd
		outBuf := new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--base", "main"}) // Set args without --yes

		err := runSquash(cmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, outBuf.String(), generatedMsg)
		assert.Contains(t, outBuf.String(), "Aborted.") // Since it's a dry run and we don't supply "y"

		mockAgent.AssertExpectations(t)
	})

	t.Run("Squash and Apply (--yes)", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(dir string) bool { return true }
		mockGit.MergeBaseFunc = func(dir, ref1, ref2 string) (string, error) { return "abcdef", nil }
		mockGit.LogFunc = func(dir string, args ...string) ([]string, error) { return []string{"12345: WIP"}, nil }
		mockGit.DiffFunc = func(dir, start, end string) (string, error) { return "diff --git a/foo b/foo", nil }

		var resetCalled bool
		var commitMsg string
		mockGit.ResetSoftFunc = func(dir, target string) error {
			resetCalled = true
			return nil
		}
		mockGit.CommitFunc = func(dir, msg string) error {
			commitMsg = msg
			return nil
		}

		generatedMsg := "feat: single commit"
		mockAgent.On("Send", mock.Anything, mock.Anything).Return(generatedMsg, nil).Once()

		// Re-initialize command because flags are global singletons
		// Reset flag variables before each test where they matter
		squashApply = true
		squashBaseBranch = "main"

		cmd := squashCmd
		outBuf := new(bytes.Buffer)
		cmd.SetOut(outBuf)
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--yes"})

		err := runSquash(cmd, []string{})
		assert.NoError(t, err)
		assert.Contains(t, outBuf.String(), generatedMsg)
		assert.Contains(t, outBuf.String(), "Commits squashed successfully!")

		assert.True(t, resetCalled)
		assert.Equal(t, generatedMsg, commitMsg)

		mockAgent.AssertExpectations(t)
	})

	t.Run("Agent Error", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(dir string) bool { return true }
		mockGit.MergeBaseFunc = func(dir, ref1, ref2 string) (string, error) { return "abcdef", nil }
		mockGit.LogFunc = func(dir string, args ...string) ([]string, error) { return []string{"12345: WIP"}, nil }
		mockGit.DiffFunc = func(dir, start, end string) (string, error) { return "diff", nil }

		mockAgent.On("Send", mock.Anything, mock.Anything).Return("", fmt.Errorf("agent error")).Once()

		cmd := squashCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--base", "main"})

		err := runSquash(cmd, []string{})
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "agent failed")
		}
	})

	t.Run("Not A Git Repo", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(dir string) bool { return false }

		cmd := squashCmd
		cmd.SetOut(new(bytes.Buffer))
		cmd.SetErr(new(bytes.Buffer))
		cmd.SetArgs([]string{"--base", "main"})

		err := runSquash(cmd, []string{})
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "not a git repository")
		}
	})
}
