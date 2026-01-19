package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	queryFocus  string
	queryIgnore []string
)

var queryCmd = &cobra.Command{
	Use:   "query [question]",
	Short: "Ask a natural language question about the codebase",
	Long: `Analyze the codebase structure and content to answer a natural language question.
This command generates a context from the codebase (respecting .gitignore and specified focus)
and uses the configured AI agent to answer your query.`,
	Args: cobra.MinimumNArgs(1),
	RunE: runQuery,
}

func init() {
	rootCmd.AddCommand(queryCmd)
	queryCmd.Flags().StringVarP(&queryFocus, "focus", "f", ".", "Focus analysis on a specific path")
	queryCmd.Flags().StringSliceVarP(&queryIgnore, "ignore", "i", nil, "Files or directories to ignore")
}

func runQuery(cmd *cobra.Command, args []string) error {
	question := args[0]
	if len(args) > 1 {
		// Join multiple args into a single question string if user didn't quote
		for _, arg := range args[1:] {
			question += " " + arg
		}
	}

	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Generate Context
	roots := []string{queryFocus}
	if queryFocus == "." {
		roots = []string{"."}
	} else {
		// Verify focus path exists
		if _, err := os.Stat(queryFocus); err != nil {
			return fmt.Errorf("focus path does not exist: %w", err)
		}
	}

	opts := ContextOptions{
		Roots:   roots,
		Ignore:  queryIgnore,
		MaxSize: 100 * 1024, // 100KB limit per file to save tokens
		Tree:    true,
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing codebase context...")
	codebaseContext, err := GenerateCodebaseContext(opts)
	if err != nil {
		return fmt.Errorf("failed to generate codebase context: %w", err)
	}

	// 2. Prepare Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-query")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	// 3. Prompt
	prompt := fmt.Sprintf(`You are an expert software engineer and codebase navigator.
Answer the following question about the codebase based on the provided context.
Be concise, accurate, and cite file names where appropriate.

QUESTION:
%s

CODEBASE CONTEXT:
%s`, question, codebaseContext)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Consulting AI agent...")
	fmt.Fprintln(cmd.OutOrStdout(), "")

	// 4. Send to Agent
	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})

	fmt.Fprintln(cmd.OutOrStdout(), "") // Newline at end

	return err
}
