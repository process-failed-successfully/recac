package main

import (
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent/prompts"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var optimizePromptsCmd = &cobra.Command{
	Use:   "optimize-prompts",
	Short: "Iteratively improve system prompts using Gym challenges",
	Long: `Uses a Meta-Agent to evolve and improve system prompts by running them against
defined Gym challenges. When a challenge fails, the Meta-Agent analyzes the logs
and rewrites the prompt to address the failure.`,
	RunE: runOptimizePrompts,
}

func init() {
	rootCmd.AddCommand(optimizePromptsCmd)
	optimizePromptsCmd.Flags().StringP("challenge", "c", "", "Path to Gym challenge file (required)")
	optimizePromptsCmd.Flags().StringP("prompt", "p", "coding_agent", "Name of the prompt to optimize")
	optimizePromptsCmd.Flags().Int("iterations", 5, "Maximum number of optimization iterations")
	optimizePromptsCmd.Flags().String("out", "", "Output file for the optimized prompt (default: overwrites in place or prints)")
	optimizePromptsCmd.MarkFlagRequired("challenge")
}

func runOptimizePrompts(cmd *cobra.Command, args []string) error {
	challengePath, _ := cmd.Flags().GetString("challenge")
	promptName, _ := cmd.Flags().GetString("prompt")
	maxIterations, _ := cmd.Flags().GetInt("iterations")
	outFile, _ := cmd.Flags().GetString("out")

	// 1. Setup Environment
	// We need a place to store the evolving prompt where GetPrompt will find it.
	// We use RECAC_PROMPTS_DIR.
	// If it's already set, we use it. If not, we create a temp dir.
	promptsDir := os.Getenv("RECAC_PROMPTS_DIR")
	if promptsDir == "" {
		tempDir, err := os.MkdirTemp("", "recac-prompts-*")
		if err != nil {
			return fmt.Errorf("failed to create temp prompts dir: %w", err)
		}
		defer os.RemoveAll(tempDir)
		promptsDir = tempDir
		os.Setenv("RECAC_PROMPTS_DIR", promptsDir)
		fmt.Printf("Using temporary prompts directory: %s\n", promptsDir)
	}

	// 2. Load Initial Prompt
	// We use GetPrompt with empty vars to get the raw template.
	currentPromptContent, err := prompts.GetPrompt(promptName, nil)
	if err != nil {
		return fmt.Errorf("failed to load initial prompt '%s': %w", promptName, err)
	}

	// Ensure it exists in the overrides dir
	promptPath := filepath.Join(promptsDir, promptName+".md")
	if err := os.WriteFile(promptPath, []byte(currentPromptContent), 0644); err != nil {
		return fmt.Errorf("failed to write prompt to overrides dir: %w", err)
	}

	// 3. Load Challenge
	challenges, err := loadChallenges(challengePath)
	if err != nil {
		return fmt.Errorf("failed to load challenge: %w", err)
	}
	if len(challenges) == 0 {
		return fmt.Errorf("no challenges found in %s", challengePath)
	}
	// For simplicity, we optimize against the first challenge, or all?
	// Optimizing against ALL is harder (multi-objective). Let's stick to the first one or assume single file=single challenge context.
	targetChallenge := challenges[0]
	fmt.Printf("Optimizing prompt '%s' against challenge '%s'\n", promptName, targetChallenge.Name)

	// 4. Initialize Meta-Agent
	ctx := cmd.Context()
	cwd, _ := os.Getwd()
	metaAgent, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), cwd, "recac-meta-optimizer")
	if err != nil {
		return fmt.Errorf("failed to create meta-agent: %w", err)
	}

	// 5. Optimization Loop
	for i := 1; i <= maxIterations; i++ {
		fmt.Printf("\n--- Iteration %d/%d ---\n", i, maxIterations)

		// Run Gym
		// Note: The session created inside runGymSessionFunc calls GetPrompt,
		// which checks RECAC_PROMPTS_DIR, so it picks up our modified file.
		result, err := runGymSessionFunc(ctx, targetChallenge)
		if err != nil {
			// System error (docker fail, etc)
			return fmt.Errorf("gym execution failed: %w", err)
		}

		if result.Passed {
			fmt.Println("✅ Challenge PASSED!")
			fmt.Println("Optimization successful.")

			if outFile != "" {
				if err := os.WriteFile(outFile, []byte(currentPromptContent), 0644); err != nil {
					return fmt.Errorf("failed to save optimized prompt: %w", err)
				}
				fmt.Printf("Saved optimized prompt to %s\n", outFile)
			} else {
				fmt.Println("\n--- Optimized Prompt ---")
				fmt.Println(currentPromptContent)
				fmt.Println("------------------------")
			}
			return nil
		}

		fmt.Printf("❌ Challenge FAILED. Analysing failure...\n")
		// fmt.Printf("Output: %s\n", result.Output) // Too verbose?

		// Construct Meta-Prompt
		metaPrompt := fmt.Sprintf(`You are an Expert AI Prompt Engineer.
Your goal is to fix the System Prompt for a Coding Agent so that it can solve a specific coding challenge.

The Coding Agent FAILED the challenge.
Here is the current System Prompt:
<system_prompt>
%s
</system_prompt>

Here is the Challenge Description:
<challenge>
%s
</challenge>

Here is the Output/Error from the failed attempt:
<output>
%s
</output>

Please rewrite the System Prompt to address the failure.
- Keep all existing placeholders (like {spec}, {history}) intact.
- Be specific in your instructions to guide the agent better.
- Do not remove core capabilities, but refine the strategy.
- Output ONLY the new System Prompt content, no markdown fencing or explanations.`,
			currentPromptContent, targetChallenge.Description, result.Output)

		fmt.Println("🧠 Meta-Agent is rewriting the prompt...")
		newPrompt, err := metaAgent.Send(ctx, metaPrompt)
		if err != nil {
			return fmt.Errorf("meta-agent failed: %w", err)
		}

		// Clean up response (remove markdown blocks if present)
		newPrompt = strings.TrimSpace(newPrompt)
		newPrompt = strings.TrimPrefix(newPrompt, "```markdown")
		newPrompt = strings.TrimPrefix(newPrompt, "```")
		newPrompt = strings.TrimSuffix(newPrompt, "```")

		currentPromptContent = newPrompt

		// Update file for next run
		if err := os.WriteFile(promptPath, []byte(currentPromptContent), 0644); err != nil {
			return fmt.Errorf("failed to update prompt file: %w", err)
		}
	}

	return fmt.Errorf("optimization failed after %d iterations", maxIterations)
}
