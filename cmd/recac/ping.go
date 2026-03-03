package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var pingCmd = &cobra.Command{
	Use:   "ping",
	Short: "Test connection to the AI provider",
	Long: `Sends a minimal prompt to the configured AI provider to verify connectivity, authentication, and measure response latency.

This is a useful diagnostic command to ensure your RECAC configuration and API keys are working correctly.`,
	RunE: runPingCmd,
}

func init() {
	rootCmd.AddCommand(pingCmd)
}

func runPingCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	displayProvider := provider
	displayModel := model
	if displayProvider == "" {
		displayProvider = "default (or unset)"
	}
	if displayModel == "" {
		displayModel = "default (or unset)"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Testing connection to provider: %s (model: %s)...\n", displayProvider, displayModel)

	cwd, _ := os.Getwd()
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-ping")
	if err != nil {
		return fmt.Errorf("failed to initialize agent client: %w", err)
	}

	start := time.Now()

	prompt := "Respond with exactly the word 'PONG'."
	resp, err := ag.Send(ctx, prompt)

	duration := time.Since(start)

	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "❌ Connection failed after %v\n", duration)
		return fmt.Errorf("agent error: %w", err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Connection successful!\n")
	fmt.Fprintf(cmd.OutOrStdout(), "Latency: %v\n", duration)
	fmt.Fprintf(cmd.OutOrStdout(), "Response: %q\n", resp)

	return nil
}
