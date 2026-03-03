package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockGitSummaryAgent for git-summary tests
type MockGitSummaryAgent struct {
	mock.Mock
}

func (m *MockGitSummaryAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockGitSummaryAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	// Simulate streaming
	response := args.String(0)
	onChunk(response)
	return response, args.Error(1)
}

func (m *MockGitSummaryAgent) Close() error {
	return nil
}

// TestHelperProcessGitSummary is used to mock git command executions
func TestHelperProcessGitSummary(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) > 0 {
		cmd, args := args[0], args[1:]
		if cmd == "git" && len(args) > 0 && args[0] == "log" {
			if os.Getenv("MOCK_EMPTY_GIT_LOG") == "1" {
				// Empty output
				os.Exit(0)
			}
			fmt.Println("1234567 feat: added cool feature")
			fmt.Println("89abcdef fix: resolved annoying bug")
			os.Exit(0)
		}
	}
	os.Exit(1)
}

func TestGitSummaryCmd_Success(t *testing.T) {
	// Setup mock agent
	mockAgent := new(MockGitSummaryAgent)
	mockAgent.On("SendStream", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return strings.Contains(prompt, "1234567 feat: added cool feature")
	}), mock.Anything).Return("- **Features**: Added cool feature.\n- **Fixes**: Resolved annoying bug.", nil)

	// Override factory
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	// Override execCommand
	oldExec := execCommand
	execCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcessGitSummary", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}
	defer func() { execCommand = oldExec }()

	// Create command
	cmd := &cobra.Command{Use: "git-summary", RunE: runGitSummary}
	cmd.SetArgs([]string{}) // Isolate args for testing

	// Capture output
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)

	// Run with args
	err := cmd.Execute()
	assert.NoError(t, err)

	// Verify output
	output := outBuf.String()
	assert.Contains(t, output, "Generating summary for the last 5 commits...")
	assert.Contains(t, output, "- **Features**: Added cool feature.")
	mockAgent.AssertExpectations(t)
}

func TestGitSummaryCmd_EmptyLog(t *testing.T) {
	// Override execCommand to return empty output
	oldExec := execCommand
	execCommand = func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcessGitSummary", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1", "MOCK_EMPTY_GIT_LOG=1"}
		return cmd
	}
	defer func() { execCommand = oldExec }()

	// Create command
	cmd := &cobra.Command{Use: "git-summary", RunE: runGitSummary}

	// Capture output
	outBuf := new(bytes.Buffer)
	cmd.SetOut(outBuf)

	// Run
	err := cmd.Execute()
	assert.NoError(t, err)

	// Verify output
	output := outBuf.String()
	assert.Contains(t, output, "No commits found in the specified range.")
}
