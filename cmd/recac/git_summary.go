package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var gitSummaryCmd = &cobra.Command{
	Use:   "git-summary [range]",
	Short: "Generate a summary of recent git commits using AI",
	Long: `Runs 'git log --oneline' for the given commit range and asks the AI agent to summarize the changes.
If no range is provided, it defaults to the last 5 commits.

Examples:
  recac git-summary
  recac git-summary HEAD~10..HEAD
  recac git-summary main..feature-branch`,
	Args: cobra.MaximumNArgs(1),
	RunE: runGitSummary,
}

func init() {
	rootCmd.AddCommand(gitSummaryCmd)
}

func runGitSummary(cmd *cobra.Command, args []string) error {
	gitArgs := []string{"log", "--oneline"}

	var commitRange string
	if len(args) > 0 {
		commitRange = args[0]
		gitArgs = append(gitArgs, commitRange)
	} else {
		commitRange = "the last 5 commits"
		gitArgs = append(gitArgs, "-n", "5")
	}
	c := execCommand("git", gitArgs...)
	c.Stderr = cmd.ErrOrStderr()
	logContent, err := c.Output()
	if err != nil {
		return fmt.Errorf("failed to run git log: %w", err)
	}

	if len(logContent) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No commits found in the specified range.")
		return nil
	}

	// Limit log size to prevent token overflow
	const maxLogSize = 50000
	truncated := false
	if len(logContent) > maxLogSize {
		logContent = logContent[:maxLogSize]
		truncated = true
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-git-summary")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a technical writer and senior software engineer.
Please provide a clear, concise, and structured summary of the following git commits.
Group the changes into logical categories such as 'Features', 'Bug Fixes', 'Refactoring', and 'Chores' if appropriate.
Highlight the most significant changes.

Commits:
%s`, string(logContent))

	if truncated {
		prompt += "\n\n(Note: The commit log was truncated due to size limits.)"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🤖 Generating summary for %s...\n\n", commitRange)

	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})

	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("agent failed to summarize commits: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "") // Trailing newline
	return nil
}
