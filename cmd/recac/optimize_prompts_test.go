package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// GymMockAgent is already defined in gym_test.go

func TestOptimizePrompts(t *testing.T) {
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

	err = runOptimizePrompts(cmd, []string{})
	assert.NoError(t, err)

	// Verify optimized prompt was saved
	optimizedContent, err := os.ReadFile(filepath.Join(tmpPromptsDir, "optimized.md"))
	assert.NoError(t, err)
	assert.Equal(t, "Improved Prompt", string(optimizedContent))
}

func TestOptimizePrompts_Failures(t *testing.T) {
	originalRunGymSessionFunc := runGymSessionFunc
	originalAgentFactory := agentClientFactory
	defer func() {
		runGymSessionFunc = originalRunGymSessionFunc
		agentClientFactory = originalAgentFactory
	}()

	tmpPromptsDir := t.TempDir()
	os.Setenv("RECAC_PROMPTS_DIR", tmpPromptsDir)
	defer os.Unsetenv("RECAC_PROMPTS_DIR")

	promptName := "test_prompt_fail"
	err := os.WriteFile(filepath.Join(tmpPromptsDir, promptName+".md"), []byte("prompt"), 0644)
	assert.NoError(t, err)

	tmpChallengeDir := t.TempDir()
	challengePath := filepath.Join(tmpChallengeDir, "challenge.yaml")
	err = os.WriteFile(challengePath, []byte("- name: Test\n  description: desc"), 0644)
	assert.NoError(t, err)

	t.Run("Loop Exhaustion", func(t *testing.T) {
		runGymSessionFunc = func(ctx context.Context, challenge GymChallenge) (*GymResult, error) {
			return &GymResult{Passed: false, Output: "Failed"}, nil
		}

		mockMetaAgent := new(GymMockAgent)
		mockMetaAgent.On("Send", mock.Anything, mock.Anything).Return("Improved Prompt", nil)

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockMetaAgent, nil
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("challenge", challengePath, "")
		cmd.Flags().String("prompt", promptName, "")
		cmd.Flags().Int("iterations", 2, "")

		err := runOptimizePrompts(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "optimization failed after 2 iterations")
	})

	t.Run("Meta Agent Failure", func(t *testing.T) {
		runGymSessionFunc = func(ctx context.Context, challenge GymChallenge) (*GymResult, error) {
			return &GymResult{Passed: false, Output: "Failed"}, nil
		}

		mockMetaAgent := new(GymMockAgent)
		mockMetaAgent.On("Send", mock.Anything, mock.Anything).Return("", fmt.Errorf("agent error"))

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockMetaAgent, nil
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("challenge", challengePath, "")
		cmd.Flags().String("prompt", promptName, "")
		cmd.Flags().Int("iterations", 2, "")

		err := runOptimizePrompts(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "meta-agent failed")
	})

	t.Run("Gym Execution Failure", func(t *testing.T) {
		runGymSessionFunc = func(ctx context.Context, challenge GymChallenge) (*GymResult, error) {
			return nil, fmt.Errorf("system error")
		}

		mockMetaAgent := new(GymMockAgent)
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return mockMetaAgent, nil
		}

		cmd := &cobra.Command{}
		cmd.Flags().String("challenge", challengePath, "")
		cmd.Flags().String("prompt", promptName, "")
		cmd.Flags().Int("iterations", 2, "")

		err := runOptimizePrompts(cmd, []string{})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "gym execution failed")
	})
}
