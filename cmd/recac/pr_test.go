package main

import (
	"bytes"
	"context"
	"os/exec"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgentForPr for PR tests
type MockAgentForPr struct {
	mock.Mock
}

func (m *MockAgentForPr) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockAgentForPr) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	return args.String(0), args.Error(1)
}

func TestRunPr(t *testing.T) {
	// Restore original variables after test
	originalExecCommand := execCommand
	originalAgentClientFactory := agentClientFactory

	defer func() {
		execCommand = originalExecCommand
		agentClientFactory = originalAgentClientFactory
		prBase = "main"
		prCreate = false
		prDraft = false
		prTitle = ""
	}()

	tests := []struct {
		name           string
		base           string
		create         bool
		draft          bool
		customTitle    string
		gitBranch      string
		gitDiff        string
		agentResponse  string
		expectedOutput string
		expectError    bool
	}{
		{
			name:      "Basic Generation",
			base:      "main",
			gitBranch: "feature/login",
			gitDiff:   "diff content",
			agentResponse: `TITLE: Add Login
DESCRIPTION:
Added login feature`,
			expectedOutput: "Title: Add Login\nDescription:\nAdded login feature",
		},
		{
			name:        "Empty Diff",
			base:        "main",
			gitBranch:   "feature/empty",
			gitDiff:     "",
			expectError: true,
		},
		{
			name:      "Custom Title",
			base:      "main",
			gitBranch: "feature/login",
			gitDiff:   "diff content",
			customTitle: "Manual Title",
			agentResponse: `TITLE: Generated Title
DESCRIPTION:
Body`,
			expectedOutput: "Title: Manual Title",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags
			prBase = tt.base
			prCreate = tt.create
			prDraft = tt.draft
			prTitle = tt.customTitle

			// Mock Agent
			mockAgent := new(MockAgentForPr)
			mockAgent.On("Send", mock.Anything, mock.Anything).Return(tt.agentResponse, nil)

			agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
				return mockAgent, nil
			}

			// Mock Exec
			execCommand = func(name string, args ...string) *exec.Cmd {
				cmd := []string{name}
				cmd = append(cmd, args...)
				cmdStr := strings.Join(cmd, " ")

				if strings.Contains(cmdStr, "rev-parse --is-inside-work-tree") {
					return exec.Command("echo", "true")
				}
				if strings.Contains(cmdStr, "branch --show-current") {
					return exec.Command("echo", tt.gitBranch)
				}
				if strings.Contains(cmdStr, "git diff") {
					if tt.gitDiff == "" {
						return exec.Command("printf", "") // Empty output
					}
					return exec.Command("echo", tt.gitDiff)
				}

				// Default fallback
				return exec.Command("echo", "")
			}

			// Capture Output
			cmd := &cobra.Command{}
			var outBuf bytes.Buffer
			cmd.SetOut(&outBuf)
			cmd.SetErr(&outBuf)

			err := runPr(cmd, []string{})

			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Contains(t, outBuf.String(), tt.expectedOutput)
			}
		})
	}
}

func TestPrCreate(t *testing.T) {
	// Restore original variables after test
	originalExecCommand := execCommand
	originalLookPathFunc := lookPathFunc
	originalAgentClientFactory := agentClientFactory

	defer func() {
		execCommand = originalExecCommand
		lookPathFunc = originalLookPathFunc
		agentClientFactory = originalAgentClientFactory
		prBase = "main"
		prCreate = false
		prDraft = false
	}()

	// Setup
	prBase = "main"
	prCreate = true
	prDraft = true

	// Mock Agent
	mockAgent := new(MockAgentForPr)
	mockAgent.On("Send", mock.Anything, mock.Anything).Return("TITLE: My PR\nDESCRIPTION:\nBody", nil)

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock LookPath
	lookPathFunc = func(file string) (string, error) {
		if file == "gh" {
			return "/usr/bin/gh", nil
		}
		return exec.LookPath(file)
	}

	// Mock Exec to verify gh call
	ghCalled := false
	execCommand = func(name string, args ...string) *exec.Cmd {
		cmdStr := name + " " + strings.Join(args, " ")

		if name == "gh" && args[0] == "pr" && args[1] == "create" {
			ghCalled = true
			// Check args
			assert.Contains(t, cmdStr, "--title My PR")
			assert.Contains(t, cmdStr, "--draft")
			assert.Contains(t, cmdStr, "--base main")
			return exec.Command("echo", "https://github.com/org/repo/pull/1")
		}

		// Git mocks
		if strings.Contains(cmdStr, "rev-parse") { return exec.Command("echo", "true") }
		if strings.Contains(cmdStr, "branch --show-current") { return exec.Command("echo", "feature-branch") }
		if strings.Contains(cmdStr, "git diff") { return exec.Command("echo", "diff") }

		return exec.Command("echo", "")
	}

	// Capture Output
	cmd := &cobra.Command{}
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := runPr(cmd, []string{})

	assert.NoError(t, err)
	assert.True(t, ghCalled, "gh pr create should be called")
	assert.Contains(t, outBuf.String(), "Creating PR on GitHub")
}
