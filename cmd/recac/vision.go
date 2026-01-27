package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	visionPrompt string
	visionOutput string
)

var visionCmd = &cobra.Command{
	Use:   "vision [image-path]",
	Short: "Analyze an image using AI (Multimodal)",
	Long:  `Send an image to the configured AI agent for analysis or code generation.
Great for "Screenshot to Code" or extracting text from images.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		imagePath := args[0]
		if _, err := os.Stat(imagePath); os.IsNotExist(err) {
			return fmt.Errorf("image file not found: %s", imagePath)
		}

		ctx := context.Background()
		cwd, _ := os.Getwd()
		provider := viper.GetString("provider")
		model := viper.GetString("model")

		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-vision")
		if err != nil {
			return fmt.Errorf("failed to create agent: %w", err)
		}

		// Check if agent supports vision
		visionAgent, ok := ag.(agent.VisionAgent)
		if !ok {
			return fmt.Errorf("the configured provider '%s' does not support vision capabilities. Please use 'gemini' or another multimodal provider", provider)
		}

		// Use default prompt if not provided
		prompt := visionPrompt
		if prompt == "" {
			prompt = "Analyze this image and provide a detailed description. If it contains UI elements, suggest how to implement them."
		}

		fmt.Fprintln(cmd.OutOrStdout(), "👀 Analyzing image...")

		resp, err := visionAgent.SendImage(ctx, prompt, imagePath)
		if err != nil {
			return fmt.Errorf("vision analysis failed: %w", err)
		}

		if visionOutput != "" {
			if err := os.WriteFile(visionOutput, []byte(resp), 0644); err != nil {
				return fmt.Errorf("failed to write output: %w", err)
			}
			fmt.Printf("Output saved to %s\n", visionOutput)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "\n--- Analysis Result ---")
			fmt.Fprintln(cmd.OutOrStdout(), resp)
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(visionCmd)
	visionCmd.Flags().StringVarP(&visionPrompt, "prompt", "p", "", "Prompt to send with the image")
	visionCmd.Flags().StringVarP(&visionOutput, "output", "o", "", "File to save the output to")
}
