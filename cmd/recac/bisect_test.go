package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// BisectMockAgent for testing
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

func (m *BisectMockAgent) GetState() interface{} {
	return nil
}

func TestBisect_Run(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(rootCmd) // Reset root flags if any
	// Reset bisect flags
	bisectGood = ""
	bisectBad = "HEAD"
	bisectCommand = ""
	bisectAICheck = ""
	bisectAutoReset = false
	bisectMaxSteps = 5
	bisectMaxSteps = 5

	// Mock execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cmdStr := name + " " + strings.Join(arg, " ")
		// t.Logf("Mock Exec: %s", cmdStr)

		if strings.Contains(cmdStr, "git diff-index") {
			return exec.Command("true")
		}
		if strings.Contains(cmdStr, "git bisect start") {
			return exec.Command("echo", "Bisect started")
		}
		if strings.Contains(cmdStr, "git bisect bad HEAD") {
			return exec.Command("echo", "Bad set")
		}
		if strings.Contains(cmdStr, "git bisect good v1") {
			// First step triggers bisecting
			return exec.Command("echo", "Bisecting: 1 revision left")
		}

		// Verification command
		if name == "sh" && strings.Contains(cmdStr, "test-cmd") {
			// Simulate failure
			return exec.Command("false")
		}

		// Bisect loop step
		if strings.Contains(cmdStr, "git bisect bad") && !strings.Contains(cmdStr, "HEAD") {
			// Loop called bad
			return exec.Command("echo", "d839123 is the first bad commit")
		}

		// Explain info
		if strings.Contains(cmdStr, "git show --stat") {
			return exec.Command("echo", "Commit Info")
		}
		if strings.Contains(cmdStr, "git show") {
			return exec.Command("echo", "diff content")
		}

		return exec.Command("echo", "unexpected: "+cmdStr)
	}
	defer func() { execCommand = exec.Command }()

	// Mock Agent
	mockAgent := new(BisectMockAgent)
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).
		Return("Explanation: It broke.", nil).
		Run(func(args mock.Arguments) {
			cb := args.Get(2).(func(string))
			cb("Explanation: It broke.")
		})

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Run
	// recac bisect --good v1 --command "test-cmd"
	output, err := executeCommand(cmd, "bisect", "--good", "v1", "--command", "test-cmd")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "Culprit Found")
	assert.Contains(t, output, "d839123")
	assert.Contains(t, output, "Explanation: It broke")
}

func TestBisect_AICheck(t *testing.T) {
	// Setup
	cmd := rootCmd
	// Reset flags manually or rely on new parsing
	bisectGood = ""
	bisectBad = "HEAD"
	bisectCommand = ""
	bisectAICheck = ""
	bisectAutoReset = false

	// Mock execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cmdStr := name + " " + strings.Join(arg, " ")
		t.Logf("Mock Exec AI: %s", cmdStr)

		if strings.Contains(cmdStr, "git diff-index") { return exec.Command("true") }
		if strings.Contains(cmdStr, "git bisect start") { return exec.Command("echo", "started") }
		if strings.Contains(cmdStr, "git bisect bad HEAD") { return exec.Command("echo", "ok") }
		if strings.Contains(cmdStr, "git bisect good") { return exec.Command("echo", "Bisecting: 1 left") }

		// Command returns ambiguous output but exit 0
		if name == "sh" {
			return exec.Command("echo", "Output says: Error 500")
		}

		// Loop: Agent says BAD, so we expect 'git bisect bad' (without arg)
		if strings.Contains(cmdStr, "git bisect bad") && !strings.Contains(cmdStr, "HEAD") {
			return exec.Command("echo", "abc1234 is the first bad commit")
		}

		if strings.Contains(cmdStr, "git show") { return exec.Command("echo", "info") }

		return exec.Command("echo", "unexpected: "+cmdStr)
	}
	defer func() { execCommand = exec.Command }()

	// Mock Agent
	mockAgent := new(BisectMockAgent)
	// First call: Ask Verdict (might be called multiple times if bisect takes steps)
	mockAgent.On("Send", mock.Anything, mock.Anything).
		Return("BAD", nil)

	// Second call: Explain
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).
		Return("Explanation", nil)

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Run
	output, err := executeCommand(cmd, "bisect", "--good", "v1", "--command", "check", "--ai-check", "Is it 500?")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "Verdict: BAD")
	assert.Contains(t, output, "Culprit Found")
	mockAgent.AssertExpectations(t)
}
