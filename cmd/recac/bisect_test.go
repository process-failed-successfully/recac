package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"recac/internal/agent"
	"testing"

	"github.com/AlecAivazis/survey/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// BisectMockAgent for testing bisect command
type BisectMockAgent struct {
	mock.Mock
}

func (m *BisectMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *BisectMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

// BisectTestMockGitClient implements IGitClient with testify/mock
type BisectTestMockGitClient struct {
	mock.Mock
}

func (m *BisectTestMockGitClient) Checkout(repoPath, commitOrBranch string) error {
	args := m.Called(repoPath, commitOrBranch)
	return args.Error(0)
}
func (m *BisectTestMockGitClient) Diff(repoPath, commitA, commitB string) (string, error) {
	args := m.Called(repoPath, commitA, commitB)
	return args.String(0), args.Error(1)
}
func (m *BisectTestMockGitClient) DiffStaged(repoPath string) (string, error) {
	args := m.Called(repoPath)
	return args.String(0), args.Error(1)
}
func (m *BisectTestMockGitClient) DiffStat(repoPath, commitA, commitB string) (string, error) {
	args := m.Called(repoPath, commitA, commitB)
	return args.String(0), args.Error(1)
}
func (m *BisectTestMockGitClient) CurrentCommitSHA(repoPath string) (string, error) {
	args := m.Called(repoPath)
	return args.String(0), args.Error(1)
}
func (m *BisectTestMockGitClient) RepoExists(repoPath string) bool {
	args := m.Called(repoPath)
	return args.Bool(0)
}
func (m *BisectTestMockGitClient) Commit(repoPath, message string) error {
	args := m.Called(repoPath, message)
	return args.Error(0)
}
func (m *BisectTestMockGitClient) Log(repoPath string, args ...string) ([]string, error) {
	// Variadic handling in mock
	callArgs := m.Called(repoPath, args)
	return callArgs.Get(0).([]string), callArgs.Error(1)
}
func (m *BisectTestMockGitClient) CurrentBranch(repoPath string) (string, error) {
	args := m.Called(repoPath)
	return args.String(0), args.Error(1)
}
func (m *BisectTestMockGitClient) CheckoutNewBranch(repoPath, branch string) error {
	args := m.Called(repoPath, branch)
	return args.Error(0)
}
func (m *BisectTestMockGitClient) BisectStart(repoPath, bad, good string) error {
	args := m.Called(repoPath, bad, good)
	return args.Error(0)
}
func (m *BisectTestMockGitClient) BisectBad(repoPath string) error {
	args := m.Called(repoPath)
	return args.Error(0)
}
func (m *BisectTestMockGitClient) BisectGood(repoPath string) error {
	args := m.Called(repoPath)
	return args.Error(0)
}
func (m *BisectTestMockGitClient) BisectSkip(repoPath string) error {
	args := m.Called(repoPath)
	return args.Error(0)
}
func (m *BisectTestMockGitClient) BisectReset(repoPath string) error {
	args := m.Called(repoPath)
	return args.Error(0)
}
func (m *BisectTestMockGitClient) BisectLog(repoPath string) ([]string, error) {
	args := m.Called(repoPath)
	return args.Get(0).([]string), args.Error(1)
}
func (m *BisectTestMockGitClient) BisectManualStart(repoPath string) error {
	args := m.Called(repoPath)
	return args.Error(0)
}

func TestBisectCmd(t *testing.T) {
	// Setup Mocks
	origGitFactory := gitClientFactory
	origAgentFactory := agentClientFactory
	origExecCommand := execCommand
	origAskOne := askOne
	origBisectMaxSteps := bisectMaxSteps

	bisectMaxSteps = 5 // Limit steps for tests

	defer func() {
		gitClientFactory = origGitFactory
		agentClientFactory = origAgentFactory
		execCommand = origExecCommand
		askOne = origAskOne
		bisectMaxSteps = origBisectMaxSteps
	}()

	mockGit := new(BisectTestMockGitClient)
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	mockAgent := new(BisectMockAgent)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestBisectHelperProcess", "--", name}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}

	cwd, _ := os.Getwd()

	t.Run("AI Check Flow", func(t *testing.T) {
		mockGit.On("RepoExists", cwd).Return(true).Once()
		// Initial check skipped because args provided
		mockGit.On("BisectStart", cwd, "sha-bad", "sha-good").Return(nil).Once()
		mockGit.On("BisectReset", cwd).Return(nil).Once() // Defer

		// Loop 1
		mockGit.On("CurrentCommitSHA", cwd).Return("sha-1", nil).Once()
		mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(p string) bool {
			return true
		})).Return("BAD", nil).Once()
		mockGit.On("BisectBad", cwd).Return(nil).Once()

		// Loop 2
		mockGit.On("CurrentCommitSHA", cwd).Return("sha-2", nil).Once()
		mockAgent.On("Send", mock.Anything, mock.Anything).Return("This looks GOOD", nil).Once()
		mockGit.On("BisectGood", cwd).Return(nil).Once()

		// Loop 3: returns sha-2 again (Convergence)
		// We use a generic return here to handle multiple calls if visited logic triggers later than expected
		mockGit.On("CurrentCommitSHA", cwd).Return("sha-2", nil)

		// Use rootCmd to ensure correct subcommand dispatch
		cmd := rootCmd
		cmd.SetOut(os.Stdout)
		cmd.SetErr(os.Stderr)

		// Reset flags on bisectCmd explicitly
		bisectCmd.Flags().Set("ai-check", "")
		bisectCmd.Flags().Set("run", "")

		cmd.SetArgs([]string{"bisect", "sha-bad", "sha-good", "--ai-check", "does it work?"})

		fmt.Println("Executing command...")
		err := cmd.Execute()
		fmt.Println("Command execution finished.")
		assert.NoError(t, err)

		mockGit.AssertExpectations(t)
		mockAgent.AssertExpectations(t)
	})

	t.Run("Run Command Flow", func(t *testing.T) {
		mockGit.ExpectedCalls = nil // Clear expectations
		mockAgent.ExpectedCalls = nil

		mockGit.On("RepoExists", cwd).Return(true).Once()
		// Initial check skipped because args provided
		mockGit.On("BisectStart", cwd, "sha-start", "sha-good").Return(nil).Once()
		mockGit.On("BisectReset", cwd).Return(nil).Once()

		// Loop 1
		mockGit.On("CurrentCommitSHA", cwd).Return("sha-1", nil).Once()
		mockGit.On("BisectGood", cwd).Return(nil).Once()

		// Loop 2
		mockGit.On("CurrentCommitSHA", cwd).Return("sha-2", nil).Once()
		mockGit.On("BisectGood", cwd).Return(nil).Once()

		// Loop 3: Convergence
		mockGit.On("CurrentCommitSHA", cwd).Return("sha-2", nil)

		cmd := rootCmd
		cmd.SetOut(os.Stdout)
		cmd.SetErr(os.Stderr)

		bisectCmd.Flags().Set("ai-check", "")
		bisectCmd.Flags().Set("run", "")

		cmd.SetArgs([]string{"bisect", "sha-start", "sha-good", "--run", "exit 0"})

		err := cmd.Execute()
		assert.NoError(t, err)

		mockGit.AssertExpectations(t)
	})

	t.Run("Manual Interactive Flow", func(t *testing.T) {
		mockGit.ExpectedCalls = nil
		mockAgent.ExpectedCalls = nil

		mockGit.On("RepoExists", cwd).Return(true).Once()
		// Initial check skipped because args provided
		mockGit.On("BisectStart", cwd, "sha-manual", "sha-good").Return(nil).Once()
		mockGit.On("BisectReset", cwd).Return(nil).Once()

		// Loop 1
		mockGit.On("CurrentCommitSHA", cwd).Return("sha-1", nil).Once()

		// Mock askOne
		askOne = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
			if _, ok := p.(*survey.Confirm); ok {
				*(response.(*bool)) = true
				return nil
			}
			return fmt.Errorf("unexpected prompt")
		}

		mockGit.On("BisectGood", cwd).Return(nil).Once()

		// Loop 2: Converge
		mockGit.On("CurrentCommitSHA", cwd).Return("sha-1", nil)

		cmd := rootCmd
		cmd.SetOut(os.Stdout)
		cmd.SetErr(os.Stderr)

		bisectCmd.Flags().Set("ai-check", "")
		bisectCmd.Flags().Set("run", "")

		cmd.SetArgs([]string{"bisect", "sha-manual", "sha-good"})

		err := cmd.Execute()
		assert.NoError(t, err)
	})
}

func TestBisectHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for i, arg := range args {
		if arg == "--" {
			args = args[i+1:]
			break
		}
	}
	if len(args) < 3 { // shell, flag, cmd
		os.Exit(0)
	}
	cmd := args[2]

	if cmd == "exit 0" {
		os.Exit(0)
	}
	if cmd == "exit 1" {
		os.Exit(1)
	}
	os.Exit(0)
}
