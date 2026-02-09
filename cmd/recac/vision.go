package main

import (
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var visionCmd = &cobra.Command{
	Use:   "vision [image_path] [optional_prompt]",
	Short: "Analyze an image using the AI agent",
	Long:  `Send an image to the configured AI agent for analysis. You can optionally provide a specific prompt.`,
	Args:  cobra.RangeArgs(1, 2),
	RunE:  runVisionCmd,
}

func runVisionCmd(cmd *cobra.Command, args []string) error {
	imagePath := args[0]
	prompt := "Describe this image."
	if len(args) > 1 {
		prompt = args[1]
	}

	// Expand home directory if needed
	if strings.HasPrefix(imagePath, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			imagePath = filepath.Join(home, imagePath[2:])
		}
	}

	// Prepare Agent
	ctx := cmd.Context()
	projectPath, _ := os.Getwd()
	projectName := filepath.Base(projectPath)

	agentClient, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), projectPath, projectName)
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Check for Vision Capability
	visionAgent, ok := agentClient.(agent.VisionAgent)
	if !ok {
		return fmt.Errorf("the configured agent provider (%s) does not support vision capabilities", viper.GetString("provider"))
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Analyzing image...")

	// Send Request
	response, err := visionAgent.SendImage(ctx, prompt, imagePath)
	if err != nil {
		return fmt.Errorf("agent failed to analyze image: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "")
	fmt.Fprintln(cmd.OutOrStdout(), response)
	return nil
}

func init() {
	rootCmd.AddCommand(visionCmd)
}
