package main

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var commitAnalyzeCmd = &cobra.Command{
	Use:   "commit-analyze [commit-hash]",
	Short: "Analyze a git commit using AI",
	Long: `Fetches the changes introduced by a specific git commit (or HEAD by default)
and uses the configured AI agent to analyze it for code quality, potential bugs,
security issues, and adherence to best practices.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCommitAnalyzeCmd,
}

func init() {
	rootCmd.AddCommand(commitAnalyzeCmd)
}

func runCommitAnalyzeCmd(cmd *cobra.Command, args []string) error {
	commitHash := "HEAD"
	if len(args) > 0 {
		commitHash = args[0]
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Fetching diff for commit: %s...\n", commitHash)

	// Fetch git diff for the given commit
	diffCmd := execCommand("git", "show", commitHash)
	diffOutput, err := diffCmd.Output()
	if err != nil {
		return fmt.Errorf("failed to fetch git diff for commit %s: %w", commitHash, err)
	}

	diffStr := string(diffOutput)
	if diffStr == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "Commit has no changes or does not exist.")
		return nil
	}

	// Prepare Prompt
	prompt := fmt.Sprintf(`You are an expert software reviewer. Analyze the following git commit diff.
Provide a clear, structured review that highlights:
- Overall summary of the changes
- Code quality and maintainability issues
- Potential bugs or edge cases
- Security considerations
- Architectural impact or drift

Here is the commit diff:
'''
%s
'''

Keep your response concise but actionable.`, diffStr)

	// Call Agent
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-commit-analyze")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Analyzing commit...")

	// Stream Response
	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Fprintln(cmd.OutOrStdout(), "") // Newline

	if err != nil {
		return fmt.Errorf("agent failed during analysis: %w", err)
	}

	return nil
}
