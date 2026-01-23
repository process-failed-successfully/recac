package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

type MockOptimizerAgent struct {
	Count int
}

func (m *MockOptimizerAgent) Send(ctx context.Context, prompt string) (string, error) {
	m.Count++
	// We return specific variations to test the selection logic
	// Variation 1: Fail
	// Variation 2: Pass (Winner)
	// Variation 3: Fail
	return fmt.Sprintf("Optimized Prompt Variation %d", m.Count), nil
}

func (m *MockOptimizerAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Send(ctx, prompt)
}

func TestOptimizePromptsCmd(t *testing.T) {
	viper.Set("provider", "mock")
	viper.Set("model", "mock-model")

	// 1. Setup Mock Agent
	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockOptimizerAgent{}, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	// 2. Setup Mock Gym Runner
	origRunGym := runGymSessionFunc
	runGymSessionFunc = func(ctx context.Context, challenge GymChallenge) (*GymResult, error) {
		// Mock logic: "Variation 2" is the winner
		// Check the RECAC_PROMPTS_DIR env var to see what prompt we are running
		promptsDir := os.Getenv("RECAC_PROMPTS_DIR")
		if promptsDir != "" {
			// Read the prompt file
			// We assume the prompt name passed in args is "planner"
			content, _ := os.ReadFile(filepath.Join(promptsDir, "planner.md"))
			sContent := string(content)
			if sContent == "Optimized Prompt Variation 2" {
				return &GymResult{Passed: true, Challenge: challenge.Name}, nil
			}
		}
		return &GymResult{Passed: false, Challenge: challenge.Name}, nil
	}
	defer func() { runGymSessionFunc = origRunGym }()

	// 3. Create dummy gym file
	tmpDir := t.TempDir()
	gymFile := filepath.Join(tmpDir, "test_challenges.yaml")
	gymContent := `
- name: Test Challenge
  description: A test challenge
`
	if err := os.WriteFile(gymFile, []byte(gymContent), 0644); err != nil {
		t.Fatalf("Failed to create gym file: %v", err)
	}

	// 4. Create dummy baseline prompt override
	baselineDir := t.TempDir()
	t.Setenv("RECAC_PROMPTS_DIR", baselineDir)
	baselineFile := filepath.Join(baselineDir, "planner.md")
	if err := os.WriteFile(baselineFile, []byte("Original Prompt"), 0644); err != nil {
		t.Fatalf("Failed to write baseline prompt: %v", err)
	}

	// 5. Run Command Logic Directly to avoid rootCmd initialization side-effects
	cmd := &cobra.Command{}
	cmd.Flags().String("gym", "", "")
	cmd.Flags().Int("iterations", 1, "")
	cmd.Flags().Int("variations", 2, "")

	cmd.Flags().Set("gym", gymFile)
	cmd.Flags().Set("iterations", "1")
	cmd.Flags().Set("variations", "3")

	if err := runOptimizePrompts(cmd, []string{"planner"}); err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	// 6. Verify Result
	outputFile := "planner_optimized.md"
	defer os.Remove(outputFile) // Clean up generated file

	content, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("Output file %s not found. Current Dir: %s", outputFile, getwd())
	}

	if string(content) != "Optimized Prompt Variation 2" {
		t.Errorf("Expected 'Optimized Prompt Variation 2', got '%s'", string(content))
	}
}

func getwd() string {
	d, _ := os.Getwd()
	return d
}
