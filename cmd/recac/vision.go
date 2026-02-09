package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var visionCmd = &cobra.Command{
	Use:   "vision [image_path] [optional_prompt]",
	Short: "Analyze an image using the AI agent",
	Long:  `Analyze an image using a multimodal AI agent (e.g., Gemini, OpenAI). You can provide an optional prompt to guide the analysis.`,
	Args:  cobra.MinimumNArgs(1),
	RunE:  runVisionCmd,
}

func runVisionCmd(cmd *cobra.Command, args []string) error {
	imagePath := args[0]
	prompt := "Describe this image."
	if len(args) > 1 {
		prompt = args[1]
	}

	// Read image file
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		return fmt.Errorf("failed to read image file: %w", err)
	}

	// Initialize Agent
	ctx := context.Background()
	projectPath, _ := os.Getwd()
	projectName := filepath.Base(projectPath)

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Default to gemini if not specified, as it's a good vision model
	if provider == "" {
		provider = "gemini"
	}

	agentClient, err := agentClientFactory(ctx, provider, model, projectPath, projectName)
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Check for Vision Capability
	visionAgent, ok := agentClient.(agent.VisionAgent)
	if !ok {
		return fmt.Errorf("provider '%s' does not support vision capabilities", provider)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Analyzing image '%s' with %s...\n", imagePath, provider)

	// Send Request
	response, err := visionAgent.SendImage(ctx, prompt, imageData)
	if err != nil {
		return fmt.Errorf("agent failed to analyze image: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "\nResponse:")
	fmt.Fprintln(cmd.OutOrStdout(), response)

	return nil
}

func init() {
	rootCmd.AddCommand(visionCmd)
}
