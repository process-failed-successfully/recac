package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
)

// MockAgentForOptimize is a mock implementation of agent.Agent
type MockAgentForOptimize struct {
	response string
}

func (m *MockAgentForOptimize) Send(ctx context.Context, prompt string) (string, error) {
	return m.response, nil
}

func (m *MockAgentForOptimize) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.response, nil
}

func TestOptimizePromptsCmd(t *testing.T) {
	// 1. Setup Environment
	tmpDir := t.TempDir()

	// Create a dummy challenge file
	challengeFile := filepath.Join(tmpDir, "challenge.yaml")
	challengeContent := `
name: Test Challenge
description: Verify the optimization loop.
language: python
test_file: test_opt.py
tests: print("DONE")
timeout: 5
`
	err := os.WriteFile(challengeFile, []byte(challengeContent), 0644)
	assert.NoError(t, err)

	// Create a dummy initial prompt in templates (simulated via overriding GetPrompt behaviour via RECAC_PROMPTS_DIR if needed,
	// but here the command sets up its own temp dir for overrides.
	// Wait, the command reads the *initial* prompt using prompts.GetPrompt.
	// We need to ensure prompts.GetPrompt returns SOMETHING.
	// Since we can't easily modify embed.FS, we can rely on the fallback or set RECAC_PROMPTS_DIR *before* running the command
	// to point to a seed prompt.

	seedPromptsDir := filepath.Join(tmpDir, "seed_prompts")
	err = os.Mkdir(seedPromptsDir, 0755)
	assert.NoError(t, err)
	os.Setenv("RECAC_PROMPTS_DIR", seedPromptsDir)
	defer os.Unsetenv("RECAC_PROMPTS_DIR")

	err = os.WriteFile(filepath.Join(seedPromptsDir, "coding_agent.md"), []byte("Initial Bad Prompt"), 0644)
	assert.NoError(t, err)

	// 2. Mock Dependencies
	originalGymFunc := runGymSessionFunc
	originalAgentFactory := agentClientFactory
	defer func() {
		runGymSessionFunc = originalGymFunc
		agentClientFactory = originalAgentFactory
	}()

	iteration := 0
	runGymSessionFunc = func(ctx context.Context, challenge GymChallenge) (*GymResult, error) {
		iteration++
		if iteration == 1 {
			// First run fails
			return &GymResult{
				Challenge: challenge.Name,
				Passed:    false,
				Output:    "Error: Agent failed to understand instructions.",
			}, nil
		}
		// Second run passes
		return &GymResult{
			Challenge: challenge.Name,
			Passed:    true,
			Output:    "Success!",
		}, nil
	}

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockAgentForOptimize{
			response: "Improved Prompt",
		}, nil
	}

	// 3. Execute Command
	// We pass the challenge file
	cmd := optimizePromptsCmd
	// Reset flags
	cmd.Flags().Set("challenge", challengeFile)
	cmd.Flags().Set("prompt", "coding_agent")
	cmd.Flags().Set("iterations", "3")
	cmd.Flags().Set("out", filepath.Join(tmpDir, "final.md"))

	// We can't call cmd.Execute() because it might parse os.Args.
	// We call RunE directly or use cmd.SetArgs
	// cmd.SetArgs([]string{"--challenge", challengeFile}) // This interacts with global flags potentially
	// Safer to call the run function directly if possible, or construct a fresh command.
	// But `optimizePromptsCmd` is a global variable.
	// Let's call the RunE function logic directly or use execute helper if available.

	err = runOptimizePrompts(cmd, []string{})
	assert.NoError(t, err)

	// 4. Verify Results
	assert.Equal(t, 2, iteration, "Should have run 2 iterations")

	// Check if final prompt was written
	finalContent, err := os.ReadFile(filepath.Join(tmpDir, "final.md"))
	assert.NoError(t, err)
	assert.Equal(t, "Improved Prompt", string(finalContent))
}
