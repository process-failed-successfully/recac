package main

import (
	"context"
	"errors"
	"fmt"
	"os"

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
				content, err = getGitDiff()
				if err != nil {
					if errors.Is(err, ErrNoChanges) {
						return errors.New("no changes detected to review")
					}
					return err
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
