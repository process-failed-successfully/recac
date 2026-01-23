package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockAgentBisect
type MockAgentBisect struct {
	mock.Mock
}

func (m *MockAgentBisect) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentBisect) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return "", nil
}

// MockBisectHelperProcess handles exec.Command mocking
func TestBisectHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	// logic to simulate exit codes based on arguments or environment
	mode := os.Getenv("MOCK_BISECT_MODE")
	if mode == "BAD" {
		os.Exit(1)
	}
	// GOOD mode exits 0
}

func TestBisectCmd(t *testing.T) {
	// Setup Mocks
	origGitFactory := gitClientFactory
	origAgentFactory := agentClientFactory
	origExec := execCommand
	defer func() {
		gitClientFactory = origGitFactory
		agentClientFactory = origAgentFactory
		execCommand = origExec
	}()

	tempDir, _ := os.MkdirTemp("", "recac-bisect-test")
	defer os.RemoveAll(tempDir)
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	// Mock Git
	mockGit := new(MockGitClient)
	gitClientFactory = func() IGitClient { return mockGit }

	// Mock Agent
	mockAgent := new(MockAgentBisect)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock Exec
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// If running the generated script
		if filepath.Base(name) == "repro.sh" {
			cs := []string{"-test.run=TestBisectHelperProcess", "--", name}
			cs = append(cs, arg...)
			cmd := exec.Command(os.Args[0], cs...)
			cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")

			state, _ := os.ReadFile(filepath.Join(tempDir, "state"))
			cmd.Env = append(cmd.Env, "MOCK_BISECT_MODE="+string(state))
			return cmd
		}
		return exec.Command(name, arg...)
	}

	t.Run("Bisect Success with Generated Script", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.CurrentCommitSHAFunc = func(path string) (string, error) { return "bad-sha", nil }

		// Checkout logic
		mockGit.CheckoutFunc = func(repoPath, commitOrBranch string) error {
			mode := "BAD"
			if commitOrBranch == "good-sha" {
				mode = "GOOD"
			}
			os.WriteFile(filepath.Join(tempDir, "state"), []byte(mode), 0644)
			return nil
		}
		// Initial state
		os.WriteFile(filepath.Join(tempDir, "state"), []byte("BAD"), 0644)

		// Agent logic
		mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
			return true
		})).Return("#!/bin/bash\nexit 1", nil).Once()

		mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
			return true
		})).Return("The bug was caused by...", nil).Once()

		// Bisect logic
		mockGit.BisectStartFunc = func(dir, bad, good string) error {
			return nil
		}
		mockGit.BisectRunFunc = func(dir, script string) (string, error) {
			return "running...\nsha-culprit is the first bad commit\n", nil
		}
		mockGit.BisectResetFunc = func(dir string) error { return nil }

		// Explain logic
		mockGit.DiffFunc = func(repo, a, b string) (string, error) { return "diff", nil }
		mockGit.LogFunc = func(repo string, args ...string) ([]string, error) { return []string{"fix: bug"}, nil }

		// Using root.Execute through executeCommand helper
		out, err := executeCommand(rootCmd, "bisect", "bug description", "--good", "good-sha")
		require.NoError(t, err, "Output: %s", out)
		assert.Contains(t, out, "Culprit found")
	})

	t.Run("Missing Good Commit", func(t *testing.T) {
		mockGit.RepoExistsFunc = func(path string) bool { return true }
		mockGit.CurrentCommitSHAFunc = func(path string) (string, error) { return "bad-sha", nil }

		_, err := executeCommand(rootCmd, "bisect", "bug description")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "provide a good commit")
	})
}
