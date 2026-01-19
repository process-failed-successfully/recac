package main

import (
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewChangelogCmd() *cobra.Command {
	var since string
	var commitRange string
	var outputFile string

	cmd := &cobra.Command{
		Use:   "changelog",
		Short: "Generate a changelog using AI",
		Long: `Generates a structured changelog (Markdown) from git commit history using the configured AI agent.
It groups commits by type (Feature, Fix, Chore, etc.) and provides a summary.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			cwd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get current working directory: %w", err)
			}

			gitClient := gitClientFactory()
			if !gitClient.RepoExists(cwd) {
				return fmt.Errorf("not a git repository")
			}

			// Determine log arguments
			logArgs := []string{"--pretty=format:%h %an: %s", "--no-merges"}
			if commitRange != "" {
				logArgs = append(logArgs, commitRange)
			} else if since != "" {
				logArgs = append(logArgs, "--since", since)
			} else {
				// Default to last 20 commits if nothing specified
				logArgs = append(logArgs, "-n", "20")
			}

			logs, err := gitClient.Log(cwd, logArgs...)
			if err != nil {
				return fmt.Errorf("failed to get git logs: %w", err)
			}

			if len(logs) == 0 {
				return fmt.Errorf("no commits found in the specified range")
			}

			// Create Agent
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-changelog")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Analyzing %d commits to generate changelog...\n", len(logs))

			prompt := fmt.Sprintf(`You are a helpful release assistant.
Generate a structured Changelog in Markdown format based on the following git commit messages.
Group them logically (e.g., ✨ Features, 🐛 Bug Fixes, 🔧 Maintenance, 📝 Documentation).
Ignore trivial or duplicate commits.
Summarize related commits into single entries where appropriate.
Do not invent features. Use only the provided logs.

Commit Logs:
%s

Output ONLY the Markdown content.`, strings.Join(logs, "\n"))

			changelog, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("failed to generate changelog: %w", err)
			}

			changelog = utils.CleanCodeBlock(changelog) // Remove potential markdown code blocks

			if outputFile != "" {
				if err := os.WriteFile(outputFile, []byte(changelog), 0644); err != nil {
					return fmt.Errorf("failed to write changelog to file: %w", err)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "Changelog written to %s\n", outputFile)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), changelog)
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&since, "since", "", "Show commits more recent than a specific date (e.g. '2 days ago')")
	cmd.Flags().StringVar(&commitRange, "range", "", "Show commits in the specified range (e.g. 'HEAD~5..HEAD', 'v1.0..v1.1')")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write output to a file instead of stdout")

	return cmd
}

var changelogCmd = NewChangelogCmd()

func init() {
	rootCmd.AddCommand(changelogCmd)
}
