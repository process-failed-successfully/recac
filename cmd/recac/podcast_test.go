package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// TestPodcastHelperProcess mimics external commands (git, which, say)
func TestPodcastHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	args := os.Args

	var cmdArgs []string
	for i, arg := range args {
		if arg == "--" {
			if i+1 < len(args) {
				cmdArgs = args[i+1:]
			}
			break
		}
	}

	if len(cmdArgs) == 0 {
		os.Exit(0)
	}

	cmd := cmdArgs[0]

	if cmd == "git" {
		if len(cmdArgs) > 1 && cmdArgs[1] == "log" {
			fmt.Print("COMMIT::abc1234|Alice|2023-10-27T10:00:00Z|Feat: Add podcast|Added podcast command\n")
		}
	} else if cmd == "which" {
		// Simulate 'say' exists
		if len(cmdArgs) > 1 && (cmdArgs[1] == "say" || cmdArgs[1] == "espeak") {
			os.Exit(0)
		} else {
			os.Exit(1)
		}
	} else if cmd == "say" || cmd == "espeak" {
		// Simulate speaking
		// We could print something to verify it ran
		fmt.Print("Speaking...")
		os.Exit(0)
	}

	os.Exit(0)
}

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

	// 2. Setup Mock Exec
	origExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestPodcastHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() { execCommand = origExecCommand }()

	// 3. Create Temp Dir
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
	// IMPORTANT: Flags must match those defined in podcast.go for cmd.Flags() to work
	cmd.Flags().String("since", "7d", "")
	cmd.Flags().String("output", "", "")
	cmd.Flags().Bool("speak", false, "")

	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)

	// Force speak flag for this test to exercise TTS logic
	cmd.SetArgs([]string{"--speak", "--since", "24h"})

	err = cmd.Execute()
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "🎧 Recording episode")
	assert.Contains(t, output, "**Alex**: Welcome to the show!")
}

func TestCleanForTTS(t *testing.T) {
	// Updated test input to match regex expectation (multiline header)
	input := "**Alex**: Hello! _Emphasis_\n# Header"
	result := cleanForTTS(input)

	assert.Contains(t, result, "Alex")
	assert.NotContains(t, result, "**")
	assert.NotContains(t, result, "_")
	assert.NotContains(t, result, "#")
	assert.NotContains(t, result, "Header")
}
