package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"recac/internal/agent"
)

var visionCmd = &cobra.Command{
	Use:   "vision <image_path> [prompt]",
	Short: "Analyze an image using a multimodal agent",
	Long:  `Analyze an image using the configured multimodal agent (e.g., Gemini, OpenAI).
You can provide an optional text prompt. If not provided, it defaults to "Describe this image."`,
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

	// Expand home directory if needed
	if len(imagePath) > 2 && imagePath[:2] == "~/" {
		home, _ := os.UserHomeDir()
		imagePath = filepath.Join(home, imagePath[2:])
	}

	// Verify image file exists
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return fmt.Errorf("image file not found: %s", imagePath)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Default provider/model if not set
	if provider == "" {
		provider = "gemini"
	}

	cwd, _ := os.Getwd()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Use the shared factory
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-vision")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	visionAgent, ok := ag.(agent.VisionAgent)
	if !ok {
		return fmt.Errorf("provider '%s' does not support vision", provider)
	}

	fmt.Printf("Analyzing image with %s (%s)...\n", provider, model)
	response, err := visionAgent.SendImage(ctx, prompt, imagePath)
	if err != nil {
		return fmt.Errorf("failed to analyze image: %w", err)
	}

	fmt.Println("\nResponse:")
	fmt.Println(response)

	return nil
}
