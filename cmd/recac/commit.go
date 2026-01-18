package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewCommitCmd() *cobra.Command {
	var apply bool

	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Generate a commit message and optionally commit changes",
		Long:  `Generates a Conventional Commit message based on staged changes using the configured AI agent.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			// 1. Get Git Client
			gitClient := gitClientFactory()
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}

			// 2. Check for staged changes
			if !gitClient.RepoExists(cwd) {
				return fmt.Errorf("not a git repository")
			}

			diff, err := gitClient.DiffStaged(cwd)
			if err != nil {
				return fmt.Errorf("failed to get staged diff: %w", err)
			}

			if strings.TrimSpace(diff) == "" {
				return fmt.Errorf("no staged changes found. Please stage changes before running recac commit")
			}

			// 3. Get Agent
			provider := viper.GetString("provider")
			model := viper.GetString("model")

			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-commit")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			// 4. Generate Message
			fmt.Fprintln(cmd.ErrOrStderr(), "Generating commit message...")

			prompt := fmt.Sprintf(`Generate a concise Conventional Commit message for the following changes.
Output ONLY the commit message (subject and optional body). Do not include backticks or markdown formatting.

Changes:
%s`, diff)

			msg, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("failed to generate commit message: %w", err)
			}

			msg = cleanCode(msg)

			fmt.Fprintln(cmd.OutOrStdout(), "Generated Commit Message:")
			fmt.Fprintln(cmd.OutOrStdout(), "-------------------------")
			fmt.Fprintln(cmd.OutOrStdout(), msg)
			fmt.Fprintln(cmd.OutOrStdout(), "-------------------------")

			// 5. Commit if requested
			if apply {
				fmt.Fprintln(cmd.ErrOrStderr(), "Committing changes...")
				if err := gitClient.Commit(cwd, msg); err != nil {
					return fmt.Errorf("commit failed: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Changes committed successfully.")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "\nTip: Run with --yes to automatically commit.")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&apply, "yes", "y", false, "Automatically commit with the generated message")
	return cmd
}

var commitCmd = NewCommitCmd()

func init() {
	rootCmd.AddCommand(commitCmd)
}
