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
	// Mock gym session: fails first, then passes
	failures := 0
	mockGymRunner := func(ctx context.Context, challenge GymChallenge, deps GymDependencies) (*GymResult, error) {
		if failures == 0 {
			failures++
			return &GymResult{Passed: false, Output: "Failed"}, nil
		}
		return &GymResult{Passed: true, Output: "Success"}, nil
	}

	// Mock Meta Agent
	mockMetaAgent := new(GymMockAgent)
	mockMetaAgent.On("Send", mock.Anything, mock.Anything).Return("Improved Prompt", nil)

	mockAgentFactory := func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockMetaAgent, nil
	}

	deps := OptimizeDependencies{
		AgentFactory: mockAgentFactory,
		GymRunner:    mockGymRunner,
		GymDeps:      defaultGymDeps,
	}

	// Create temp dir for prompts
	tmpPromptsDir := t.TempDir()
	os.Setenv("RECAC_PROMPTS_DIR", tmpPromptsDir)
	defer os.Unsetenv("RECAC_PROMPTS_DIR")

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

	err = runOptimizePromptsWithDeps(cmd, []string{}, deps)
	assert.NoError(t, err)

	// Verify optimized prompt was saved
	optimizedContent, err := os.ReadFile(filepath.Join(tmpPromptsDir, "optimized.md"))
	assert.NoError(t, err)

	assert.Equal(t, "Improved Prompt", string(optimizedContent))
}
