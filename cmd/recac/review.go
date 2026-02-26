package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"recac/internal/utils"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var reviewInteractive bool

// runReviewTUIFunc is a variable that allows mocking the TUI execution in tests.
var runReviewTUIFunc = func(m tea.Model) error {
	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running review TUI: %w", err)
	}
	return nil
}

type ReviewIssue struct {
	File            string `json:"file"`
	Line            int    `json:"line"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Severity        string `json:"severity"` // "CRITICAL", "WARNING", "INFO"
	Suggestion      string `json:"suggestion"`
	Replacement     string `json:"replacement,omitempty"`
	OriginalContent string `json:"original_content,omitempty"`
}

func NewReviewCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review [file]",
		Short: "Review code or changes using AI",
		Long:  `Reviews a specific file or current git changes (diff) using the configured AI agent.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			content, sourceDescription, err := getReviewContent(args)
			if err != nil {
				return err
			}

			if len(content) == 0 {
				return errors.New("content is empty")
			}

			if reviewInteractive {
				return runInteractiveReview(cmd, content, sourceDescription)
			}

			return runStreamReview(cmd, content, sourceDescription)
		},
	}
	cmd.Flags().BoolVarP(&reviewInteractive, "interactive", "i", false, "Enable interactive TUI mode")
	return cmd
}

var reviewCmd = NewReviewCmd()

func init() {
	rootCmd.AddCommand(reviewCmd)
}

func getReviewContent(args []string) (string, string, error) {
	if len(args) > 0 {
		// Review specific file
		filePath := args[0]
		fileContent, err := os.ReadFile(filePath)
		if err != nil {
			return "", "", fmt.Errorf("failed to read file %s: %w", filePath, err)
		}
		return string(fileContent), fmt.Sprintf("file: %s", filePath), nil
	}

	// Review git changes
	diffCmd := exec.Command("git", "diff", "HEAD")
	var out bytes.Buffer
	diffCmd.Stdout = &out
	if err := diffCmd.Run(); err != nil {
		// Fallback for fresh repo or no HEAD
		diffCmd = exec.Command("git", "diff")
		out.Reset()
		diffCmd.Stdout = &out
		if err := diffCmd.Run(); err != nil {
			return "", "", fmt.Errorf("failed to get git diff: %w", err)
		}
	}
	content := out.String()
	if len(content) == 0 {
		return "", "", errors.New("no changes detected to review")
	}
	return content, "current git changes", nil
}

func runStreamReview(cmd *cobra.Command, content, sourceDescription string) error {
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-review")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf("Please review the following %s for bugs, security issues, and style improvements. Be concise and prioritize critical issues:\n\n```\n%s\n```", sourceDescription, content)

	fmt.Fprintf(cmd.OutOrStdout(), "Reviewing %s...\n\n", sourceDescription)

	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Fprintln(cmd.OutOrStdout(), "")

	return err
}

func runInteractiveReview(cmd *cobra.Command, content, sourceDescription string) error {
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-review-interactive")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a senior software engineer conducting a code review.
Analyze the following %s.
Identify bugs, security issues, performance bottlenecks, and code style improvements.

Return a raw JSON list of objects with the following structure:
[
  {
    "file": "path/to/file",
    "line": <line_number>,
    "title": "Short summary",
    "description": "Detailed explanation",
    "severity": "CRITICAL|WARNING|INFO",
    "suggestion": "The corrected code block (if applicable) or a description of the fix"
  }
]

Do NOT verify the file existence, trust the provided paths.
Do NOT wrap the JSON in markdown code blocks. Just return the raw JSON string.

CODE TO REVIEW:
%s`, sourceDescription, content)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Analyzing changes for interactive review...")
	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed to generate review: %w", err)
	}

	jsonStr := utils.CleanJSONBlock(resp)
	var issues []ReviewIssue
	if err := json.Unmarshal([]byte(jsonStr), &issues); err != nil {
		// Attempt to recover if it's not perfect JSON
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to parse JSON response: %v\n", err)
		fmt.Fprintln(cmd.OutOrStdout(), "Raw response:")
		fmt.Fprintln(cmd.OutOrStdout(), resp)
		return nil
	}

	if len(issues) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No issues found!")
		return nil
	}

	// Filter issues without valid file paths if possible, or just pass them to TUI
	// The TUI will handle display.

	// Run TUI
	return runReviewTUIFunc(initialReviewModel(issues))
}
