package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestOptimizePrompts(t *testing.T) {
	gymTestMutex.Lock()
	defer gymTestMutex.Unlock()

	// Mock factories
	originalRunGymSessionFunc := runGymSessionFunc
	originalAgentFactory := agentClientFactory
	defer func() {
		runGymSessionFunc = originalRunGymSessionFunc
		agentClientFactory = originalAgentFactory
	}()

	// Mock gym session: fails first, then passes
	failures := 0
	runGymSessionFunc = func(ctx context.Context, challenge GymChallenge) (*GymResult, error) {
		if failures == 0 {
			failures++
			return &GymResult{Passed: false, Output: "Failed"}, nil
		}
		return &GymResult{Passed: true, Output: "Success"}, nil
	}

	// Mock Meta Agent
	mockMetaAgent := new(GymMockAgent)
	mockMetaAgent.On("Send", mock.Anything, mock.Anything).Return("Improved Prompt", nil)

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockMetaAgent, nil
	}

	// Create temp dir for prompts
	tmpPromptsDir := t.TempDir()
	t.Setenv("RECAC_PROMPTS_DIR", tmpPromptsDir)

	// Create initial prompt file
	promptName := "test_prompt"
	initialPrompt := "Initial Prompt Content"
	err := os.WriteFile(filepath.Join(tmpPromptsDir, promptName+".md"), []byte(initialPrompt), 0644)
	assert.NoError(t, err)

	// Create temp challenge file
	tmpChallengeDir := t.TempDir()
	challengePath := filepath.Join(tmpChallengeDir, "challenge.yaml")
	err = os.WriteFile(challengePath, []byte("- name: Test\n  description: desc"), 0644)
	assert.NoError(t, err)

	// Run command
	cmd := &cobra.Command{}
	cmd.Flags().String("challenge", challengePath, "")
	cmd.Flags().String("prompt", promptName, "")
	cmd.Flags().Int("iterations", 5, "")
	cmd.Flags().String("out", filepath.Join(tmpPromptsDir, "optimized.md"), "")

	// Override RunE to call our function directly or just call runOptimizePrompts directly
	// Since runOptimizePrompts is not exported, we can call it if we are in package main_test (which we are if in same dir)
	// But go test -v ./cmd/recac will compile main package test.
	// We need to be in package main.

	err = runOptimizePrompts(cmd, []string{})
	assert.NoError(t, err)

	// Verify optimized prompt was saved
	optimizedContent, err := os.ReadFile(filepath.Join(tmpPromptsDir, "optimized.md"))
	assert.NoError(t, err)

	// In the test:
	// 1. Initial prompt loaded from file: "Initial Prompt Content"
	// 2. Gym fails.
	// 3. Meta Agent returns "Improved Prompt".
	// 4. "Improved Prompt" written to promptPath.
	// 5. Gym passes.
	// 6. "Improved Prompt" written to outFile.
	assert.Equal(t, "Improved Prompt", string(optimizedContent))
}
