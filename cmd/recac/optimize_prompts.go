package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/agent"
	"recac/internal/agent/prompts"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var optimizePromptsCmd = &cobra.Command{
	Use:   "optimize-prompts [prompt_name]",
	Short: "Optimize a system prompt using an evolutionary approach",
	Long: `Uses a meta-agent to iteratively improve a system prompt by running it against gym challenges.
It generates variations, evaluates them, and keeps the best performing one.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runOptimizePrompts,
}

func init() {
	optimizePromptsCmd.Flags().String("gym", "./gym/challenges.yaml", "Path to gym challenges file")
	optimizePromptsCmd.Flags().Int("iterations", 3, "Number of optimization iterations")
	optimizePromptsCmd.Flags().Int("variations", 2, "Number of variations per iteration")
	rootCmd.AddCommand(optimizePromptsCmd)
}

func runOptimizePrompts(cmd *cobra.Command, args []string) error {
	promptName := args[0]
	gymPath, _ := cmd.Flags().GetString("gym")
	iterations, _ := cmd.Flags().GetInt("iterations")
	variations, _ := cmd.Flags().GetInt("variations")

	// 1. Load Baseline Prompt
	// GetPrompt with nil vars returns the raw template (placeholders preserved)
	baselineContent, err := prompts.GetPrompt(promptName, nil)
	if err != nil {
		return fmt.Errorf("failed to load baseline prompt '%s': %w", promptName, err)
	}

	fmt.Printf("Optimizing prompt: %s\n", promptName)
	fmt.Printf("Baseline length: %d chars\n", len(baselineContent))

	// 2. Initialize Optimizer Agent
	ctx := context.Background()
	cwd, _ := os.Getwd()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	optimizerAgent, err := agentClientFactory(ctx, provider, model, cwd, "recac-optimizer")
	if err != nil {
		return fmt.Errorf("failed to create optimizer agent: %w", err)
	}

	// 3. Load Gym Challenges
	challenges, err := loadChallenges(gymPath)
	if err != nil {
		return fmt.Errorf("failed to load gym challenges: %w", err)
	}
	if len(challenges) == 0 {
		return fmt.Errorf("no challenges found in %s", gymPath)
	}

	bestContent := baselineContent
	bestScore := evaluatePrompt(ctx, promptName, baselineContent, challenges)
	fmt.Printf("Baseline Score: %.2f%% (Pass: %d/%d)\n", bestScore.PassRate*100, bestScore.Passed, bestScore.Total)

	for i := 0; i < iterations; i++ {
		fmt.Printf("\n--- Iteration %d/%d ---\n", i+1, iterations)

		for v := 0; v < variations; v++ {
			fmt.Printf("Generating variation %d...\n", v+1)
			variationContent, err := generateVariation(ctx, optimizerAgent, bestContent, promptName, bestScore)
			if err != nil {
				fmt.Printf("Failed to generate variation: %v\n", err)
				continue
			}

			score := evaluatePrompt(ctx, promptName, variationContent, challenges)
			fmt.Printf("Variation %d Score: %.2f%% (Pass: %d/%d)\n", v+1, score.PassRate*100, score.Passed, score.Total)

			// Simple hill climbing: strict improvement or same score with shorter prompt (efficiency)
			if score.PassRate > bestScore.PassRate || (score.PassRate == bestScore.PassRate && len(variationContent) < len(bestContent)) {
				fmt.Printf("New Best Found! (Improvement: %.2f%% -> %.2f%%)\n", bestScore.PassRate*100, score.PassRate*100)
				bestScore = score
				bestContent = variationContent
			}
		}
	}

	// 4. Save Result
	outputPath := fmt.Sprintf("%s_optimized.md", promptName)
	if err := os.WriteFile(outputPath, []byte(bestContent), 0644); err != nil {
		return err
	}
	fmt.Printf("\nOptimization Complete. Best prompt saved to %s\n", outputPath)
	fmt.Printf("To use this prompt, set RECAC_PROMPTS_DIR to the directory containing this file.\n")

	return nil
}

type Score struct {
	PassRate float64
	Passed   int
	Total    int
}

func evaluatePrompt(ctx context.Context, promptName, content string, challenges []GymChallenge) Score {
	// 1. Setup Override
	tmpDir, err := os.MkdirTemp("", "recac-opt-*")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		return Score{}
	}
	defer os.RemoveAll(tmpDir)

	if err := os.WriteFile(filepath.Join(tmpDir, promptName+".md"), []byte(content), 0644); err != nil {
		fmt.Printf("Error writing prompt override: %v\n", err)
		return Score{}
	}

	// Set Env (Global - Not thread safe, so we run variations sequentially)
	originalEnv := os.Getenv("RECAC_PROMPTS_DIR")
	os.Setenv("RECAC_PROMPTS_DIR", tmpDir)
	defer func() {
		if originalEnv != "" {
			os.Setenv("RECAC_PROMPTS_DIR", originalEnv)
		} else {
			os.Unsetenv("RECAC_PROMPTS_DIR")
		}
	}()

	// 2. Run Gym
	passed := 0
	for _, c := range challenges {
		// reuse runGymSession from gym.go
		res, err := runGymSessionFunc(ctx, c)
		if err == nil && res.Passed {
			passed++
		}
	}

	return Score{
		PassRate: float64(passed) / float64(len(challenges)),
		Passed:   passed,
		Total:    len(challenges),
	}
}

func generateVariation(ctx context.Context, ag agent.Agent, currentContent, promptName string, currentScore Score) (string, error) {
	prompt := fmt.Sprintf(`You are an expert Prompt Engineer optimizing a system prompt for an autonomous coding agent.
The current prompt for '%s' achieves a pass rate of %.2f%% on the test suite.

Current Prompt:
"""
%s
"""

Your task is to rewrite this prompt to improve its performance, clarity, and robustness.
Keep the placeholders (e.g. {spec}, {history}) intact.
Do not remove critical instructions, but refine them.
Return ONLY the new prompt content, without markdown code blocks or explanations.
`, promptName, currentScore.PassRate*100, currentContent)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return "", err
	}

	// Clean up response
	resp = strings.TrimSpace(resp)
	resp = strings.TrimPrefix(resp, "```markdown")
	resp = strings.TrimPrefix(resp, "```")
	resp = strings.TrimSuffix(resp, "```")
	return resp, nil
}
