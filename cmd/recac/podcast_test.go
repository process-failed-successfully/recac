package main

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestPodcastCmd(t *testing.T) {
	// 1. Setup Mock Agent
	mockAgent := new(MockAgent)
	expectedScript := "**Alex**: Welcome to the show!\n**Sam**: Let's see what broke today."
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(expectedScript, nil)

	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	// 2. Setup Mock Exec using simple echo/true commands to avoid helper process complexity
	origExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Mock git log
		if name == "git" && len(arg) > 0 && arg[0] == "log" {
			// We return a command that prints the expected commit log
			return exec.Command("echo", "COMMIT::abc1234|Alice|2023-10-27T10:00:00Z|Feat: Add podcast|Added podcast command")
		}

		// Mock which (check for say/espeak)
		if name == "which" {
			return exec.Command("true") // Simulate found
		}

		// Mock say/espeak
		if name == "say" || name == "espeak" {
			return exec.Command("true") // Simulate success
		}

		// Fallback to echo for safety if unexpected
		return exec.Command("echo", "mock exec: "+name)
	}
	defer func() { execCommand = origExecCommand }()

	// 3. Create Temp Dir with Dummy Code
	tempDir, err := os.MkdirTemp("", "podcast-test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	// Create dummy files
	os.WriteFile("main.go", []byte("package main\nfunc main() {}\n"), 0644)
	os.WriteFile("README.md", []byte("# Test Project"), 0644)

	// 4. Run Command
	cmd := &cobra.Command{Use: "podcast", RunE: runPodcast}
	cmd.Flags().String("since", "24h", "")
	cmd.Flags().String("output", "", "")
	cmd.Flags().Bool("speak", true, "")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Force speak flag
	cmd.Flags().Set("speak", "true")

	err = runPodcast(cmd, []string{})
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "🎧 Recording episode")
	assert.Contains(t, output, "**Alex**: Welcome to the show!")
}

func TestCleanForTTS(t *testing.T) {
	input := "**Alex**: Hello! _Emphasis_ # Header"
	result := cleanForTTS(input)

	assert.Contains(t, result, "Alex")
	assert.NotContains(t, result, "**")
	assert.NotContains(t, result, "_")
	assert.NotContains(t, result, "#")
}
