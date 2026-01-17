package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewReviewCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "review [file]",
		Short: "Review code or changes using AI",
		Long:  `Reviews a specific file or current git changes (diff) using the configured AI agent.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			var content string
			var sourceDescription string
			var err error

			if len(args) > 0 {
				// Review specific file
				filePath := args[0]
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					return fmt.Errorf("failed to read file %s: %w", filePath, err)
				}
				content = string(fileContent)
				sourceDescription = fmt.Sprintf("file: %s", filePath)
			} else {
				// Review git changes
				// Try git diff HEAD (changes since last commit, including staged and unstaged)
				diffCmd := exec.Command("git", "diff", "HEAD")
				var out bytes.Buffer
				diffCmd.Stdout = &out
				// We ignore stderr for now or maybe log it if verbose
				if err := diffCmd.Run(); err != nil {
					// Fallback: maybe no HEAD (fresh repo)?
					// Try just "git diff" (unstaged changes)
					// And "git diff --cached" might fail if no HEAD, but let's try just "git diff" for now
					diffCmd = exec.Command("git", "diff")
					out.Reset()
					diffCmd.Stdout = &out
					if err := diffCmd.Run(); err != nil {
						return fmt.Errorf("failed to get git diff: %w", err)
					}

					// If we are in fresh repo, we might also want staged changes which `git diff` misses.
					// But `git diff --cached` fails without HEAD.
					// So effectively in a fresh repo without commits, `git diff` only sees unstaged.
					// If user staged files in fresh repo, we might miss them.
					// Ideally we'd list cached files and show content, but that's complex.
					// We'll stick to this best-effort.
				}
				content = out.String()
				if len(content) == 0 {
					// Try one more: maybe user has staged changes but `git diff HEAD` failed?
					// Or `git diff HEAD` worked but returned empty (clean state).
					// If clean state, check if we want to say "No changes".
					return errors.New("no changes detected to review")
				}
				sourceDescription = "current git changes"
			}

			if len(content) == 0 {
				return errors.New("content is empty")
			}

			ctx := context.Background()
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			cwd, _ := os.Getwd()

			// Use the factory to create the agent
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
		},
	}
}

var reviewCmd = NewReviewCmd()

func init() {
	rootCmd.AddCommand(reviewCmd)
}
