package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	askIgnore  []string
	askMaxSize int64
)

var askCmd = &cobra.Command{
	Use:   "ask \"question\"",
	Short: "Ask a question about the codebase",
	Long:  `Ask a natural language question about the codebase. The command will aggregate the project context (file tree and contents) and use the configured AI agent to provide an answer.`,
	Args:  cobra.ExactArgs(1),
	RunE:  runAskCmd,
}

func runAskCmd(cmd *cobra.Command, args []string) error {
	question := args[0]

	// Generate Context
	opts := ContextOptions{
		Roots:     []string{"."},
		Ignore:    askIgnore,
		MaxSize:   askMaxSize,
		Tree:      true,
		NoContent: false,
	}

	fmt.Fprintln(cmd.OutOrStdout(), "Analyzing codebase...")
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// Prepare Agent
	ctx := context.Background()
	projectPath, _ := os.Getwd()
	projectName := filepath.Base(projectPath)

	agentClient, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), projectPath, projectName)
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// Construct Prompt
	prompt := fmt.Sprintf(`You are a helpful expert software engineer.
You are given the context of a codebase (file tree and contents).
Answer the user's question based on this context.
Be concise and specific.

CONTEXT:
%s

QUESTION:
%s`, codebaseContext, question)

	fmt.Fprintln(cmd.OutOrStdout(), "Consulting Agent...")
	fmt.Fprintln(cmd.OutOrStdout(), "") // Newline

	// Stream Response
	_, err = agentClient.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Fprintln(cmd.OutOrStdout(), "") // Newline at end

	if err != nil {
		return fmt.Errorf("agent failed to answer: %w", err)
	}

	return nil
}

func init() {
	askCmd.Flags().StringSliceVarP(&askIgnore, "ignore", "i", nil, "Additional ignore patterns")
	askCmd.Flags().Int64VarP(&askMaxSize, "max-size", "s", 1024*1024, "Max file size to include (bytes)")
	rootCmd.AddCommand(askCmd)
}
