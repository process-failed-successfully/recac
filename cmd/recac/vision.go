package main

import (
	"context"
	"fmt"
	"os"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var visionCmd = &cobra.Command{
	Use:   "vision [image-path] [prompt]",
	Short: "Analyze an image using AI",
	Long:  `Uploads an image (and optional prompt) to a vision-capable AI model (e.g., Gemini Pro Vision, GPT-4o) and prints the analysis.`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runVision,
}

func init() {
	rootCmd.AddCommand(visionCmd)
}

func runVision(cmd *cobra.Command, args []string) error {
	imagePath := args[0]
	prompt := "Describe this image."
	if len(args) > 1 {
		prompt = args[1]
	}

	// Verify file exists
	if _, err := os.Stat(imagePath); err != nil {
		return fmt.Errorf("image file not found: %w", err)
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-vision")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	// Check if agent supports vision
	visionAgent, ok := ag.(agent.VisionAgent)
	if !ok {
		return fmt.Errorf("provider '%s' does not support vision capabilities", provider)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "👀 Analyzing image '%s'...\n", imagePath)

	resp, err := visionAgent.SendImage(ctx, prompt, imagePath)
	if err != nil {
		return fmt.Errorf("vision analysis failed: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nAnalysis:")
	fmt.Fprintln(cmd.OutOrStdout(), resp)

	return nil
}
