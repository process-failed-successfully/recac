package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var diffExplainCmd = &cobra.Command{
	Use:   "diff-explain [git diff args...]",
	Short: "Explain a git diff using AI",
	Long: `Runs 'git diff' with the provided arguments (or reads a diff from stdin) and asks the AI to explain the changes.

Examples:
  recac diff-explain HEAD~1
  recac diff-explain main...feature-branch
  git diff | recac diff-explain`,
	RunE: runDiffExplain,
}

func init() {
	rootCmd.AddCommand(diffExplainCmd)
}

// checkPipedInput is a variable to allow mocking in tests
var checkPipedInput = func() bool {
	stat, _ := os.Stdin.Stat()
	return (stat.Mode() & os.ModeCharDevice) == 0
}

func runDiffExplain(cmd *cobra.Command, args []string) error {
	var diffContent []byte
	var err error

	// Check if data is being piped to stdin
	isPiped := checkPipedInput()

	// If the user explicitly provides stdin via SetIn (for tests), we should treat it as piped/available
	if f, ok := cmd.InOrStdin().(*os.File); ok && f != os.Stdin {
		// It's a file (or pipe) but not the real os.Stdin, so we assume it's valid input for tests
		isPiped = true
	} else if _, ok := cmd.InOrStdin().(*bytes.Buffer); ok {
		// It's a buffer (tests using SetIn with buffer)
		isPiped = true
	}

	if isPiped {
		diffContent, err = io.ReadAll(cmd.InOrStdin())
		if err != nil {
			return fmt.Errorf("failed to read from stdin: %w", err)
		}
	} else {
		// If not piped, run git diff with provided args
		if len(args) == 0 {
			// If no args and no pipe, default to "git diff" (unstaged changes)
			// checking if there are staged changes might be useful too, but let's stick to standard git diff behavior
		}

		gitArgs := append([]string{"diff"}, args...)
		c := exec.Command("git", gitArgs...)
		// inherit stderr so user sees git errors
		c.Stderr = cmd.ErrOrStderr()
		diffContent, err = c.Output()
		if err != nil {
			return fmt.Errorf("failed to run git diff: %w", err)
		}
	}

	if len(diffContent) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No changes detected or empty diff.")
		return nil
	}

	// Limit diff size to prevent token overflow (simple truncation for now)
	const maxDiffSize = 50000
	truncated := false
	if len(diffContent) > maxDiffSize {
		diffContent = diffContent[:maxDiffSize]
		truncated = true
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-diff-explain")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a senior software engineer reviewing a code change.
Please provide a clear and concise explanation of the following diff.
Focus on the "why" and "what", highlighting any major architectural changes or potential risks.

Diff:
%s`, string(diffContent))

	if truncated {
		prompt += "\n\n(Note: The diff was truncated due to size limits.)"
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Analyzing diff...")

	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})

	if err != nil && !errors.Is(err, io.EOF) {
		return fmt.Errorf("agent failed to explain diff: %w", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "") // Trailing newline
	return nil
}
