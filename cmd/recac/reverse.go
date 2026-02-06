package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"recac/internal/agent/prompts"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var reverseCmd = &cobra.Command{
	Use:   "reverse",
	Short: "Reverse-engineer app_spec.txt from codebase",
	Long:  "Analyzes the current codebase and generates a high-level application specification (app_spec.txt) suitable for rebuilding or documenting the project.",
	Run:   runReverseCmd,
}

func init() {
	rootCmd.AddCommand(reverseCmd)
	reverseCmd.Flags().String("path", ".", "Path to the codebase to analyze")
	reverseCmd.Flags().String("output", "app_spec.txt", "Output file path")
	reverseCmd.Flags().String("model", "", "AI model to use (defaults to config)")
	reverseCmd.Flags().Int64("max-size", 1024*1024, "Max size of codebase to analyze in bytes")
	reverseCmd.Flags().StringSlice("ignore", []string{}, "Additional patterns to ignore")
}

func runReverseCmd(cmd *cobra.Command, args []string) {
	ctx := context.Background()

	path, _ := cmd.Flags().GetString("path")
	output, _ := cmd.Flags().GetString("output")
	maxSize, _ := cmd.Flags().GetInt64("max-size")
	ignore, _ := cmd.Flags().GetStringSlice("ignore")

	absPath, _ := filepath.Abs(path)

	fmt.Printf("Analyzing codebase at %s...\n", absPath)

	// 1. Generate Context
	opts := ContextOptions{
		Roots:   []string{absPath},
		MaxSize: maxSize,
		Ignore:  ignore,
		Tree:    true,
	}

	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error generating codebase context: %v\n", err)
		exit(1)
	}

	fmt.Printf("Generated context (%d bytes). Initializing Agent...\n", len(codebaseContext))

	// 2. Initialize Agent
	provider := viper.GetString("provider")
	model, _ := cmd.Flags().GetString("model")
	if model == "" {
		model = viper.GetString("model")
	}

	// Use factory for testability
	ag, err := agentClientFactory(ctx, provider, model, absPath, "recac-reverse")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error initializing agent: %v\n", err)
		exit(1)
	}

	// 3. Prepare Prompt
	prompt, err := prompts.GetPrompt(prompts.ReverseEngineer, map[string]string{
		"codebase": codebaseContext,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading prompt: %v\n", err)
		exit(1)
	}

	// 4. Send to Agent
	fmt.Println("Sending request to AI... (this may take a minute)")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Agent error: %v\n", err)
		exit(1)
	}

	// 5. Save Output
	if err := os.WriteFile(output, []byte(resp), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing output file: %v\n", err)
		exit(1)
	}

	fmt.Printf("Success! Specification written to %s\n", output)
}
