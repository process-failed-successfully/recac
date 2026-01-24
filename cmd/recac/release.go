package main

import (
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func NewReleaseCmd() *cobra.Command {
	var apply bool
	var push bool
	var force bool

	cmd := &cobra.Command{
		Use:   "release",
		Short: "Automate the release process using AI",
		Long: `Analyzes git commits since the last tag, suggests the next semantic version, generates a changelog, and creates a git tag.

It can also push the tag to the remote repository.`,
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

			// 1. Fetch tags to ensure we have the latest
			fmt.Fprintln(cmd.ErrOrStderr(), "Fetching tags...")
			// We use Fetch directly.
			// Ideally we should use a method that handles output suppression if needed, but Log uses exec directly in some places.
			// Let's assume Fetch works.
			if err := gitClient.Fetch(cwd, "origin", "--tags"); err != nil {
				// Don't fail hard if fetch fails (maybe no remote), just warn
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to fetch tags: %v\n", err)
			}

			// 2. Get Last Tag
			lastTag, err := gitClient.LatestTag(cwd)
			if err != nil {
				return fmt.Errorf("failed to get latest tag: %w", err)
			}
			if lastTag == "" {
				lastTag = "v0.0.0"
				fmt.Fprintln(cmd.ErrOrStderr(), "No tags found. Assuming v0.0.0 start.")
			} else {
				fmt.Fprintf(cmd.ErrOrStderr(), "Current version: %s\n", lastTag)
			}

			// 3. Get Commits since last tag
			logRange := fmt.Sprintf("%s..HEAD", lastTag)
			// If lastTag is v0.0.0 (virtual), and it doesn't exist in git, we just get all logs.
			// But wait, "v0.0.0..HEAD" will fail if v0.0.0 is not a ref.
			var logArgs []string
			if lastTag == "v0.0.0" {
				logArgs = []string{"--pretty=format:%h %an: %s", "--no-merges"}
			} else {
				logArgs = []string{"--pretty=format:%h %an: %s", "--no-merges", logRange}
			}

			logs, err := gitClient.Log(cwd, logArgs...)
			if err != nil {
				return fmt.Errorf("failed to get git logs: %w", err)
			}

			if len(logs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No new commits since last release.")
				return nil
			}

			// 4. AI Analysis
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-release")
			if err != nil {
				return fmt.Errorf("failed to create agent: %w", err)
			}

			fmt.Fprintf(cmd.ErrOrStderr(), "Analyzing %d commits...\n", len(logs))

			prompt := fmt.Sprintf(`You are a Release Manager.
Current Version: %s
Commits since last release:
%s

Task:
1. Determine the next Semantic Version (Major, Minor, or Patch) based on the commits (e.g. BREAKING CHANGE or feat! -> Major, feat -> Minor, fix -> Patch).
2. Generate a Changelog in Markdown.

Output Format:
The first line MUST be the new version number (e.g. v1.1.0).
The rest of the output MUST be the Changelog.

Example Output:
v1.1.0
## v1.1.0 (2023-10-27)
### Features
- ...
`, lastTag, strings.Join(logs, "\n"))

			resp, err := ag.Send(ctx, prompt)
			if err != nil {
				return fmt.Errorf("failed to generate release info: %w", err)
			}

			resp = utils.CleanCodeBlock(resp)
			lines := strings.SplitN(resp, "\n", 2)
			if len(lines) < 2 {
				return fmt.Errorf("invalid response from agent")
			}

			newVersion := strings.TrimSpace(lines[0])
			changelog := strings.TrimSpace(lines[1])

			// Validate version format (basic check)
			if !strings.HasPrefix(newVersion, "v") {
				// Maybe AI forgot 'v', let's add it if it looks like numbers
				newVersion = "v" + newVersion
			}

			fmt.Fprintln(cmd.OutOrStdout(), "Proposed Release:")
			fmt.Fprintf(cmd.OutOrStdout(), "Version: %s -> %s\n", lastTag, newVersion)
			fmt.Fprintln(cmd.OutOrStdout(), "\nChangelog:")
			fmt.Fprintln(cmd.OutOrStdout(), changelog)
			fmt.Fprintln(cmd.OutOrStdout(), "------------------------------------------------")

			// 5. Confirmation / Execution
			if !apply {
				if force {
					apply = true
				} else {
					fmt.Fprint(cmd.OutOrStdout(), "Proceed with this release? [y/N]: ")
					var confirm string
					// Check if running in test with mocked input
					if _, ok := cmd.InOrStdin().(*os.File); !ok {
						// It's a buffer or pipe
						// If we can't scan, we assume no
					}
					// Use Fscanln on InOrStdin if possible, but Fscanln takes io.Reader.
					// cmd.InOrStdin() returns io.Reader.
					// But we need to handle potential errors if it's empty (like in tests without buffer).
					_, err := fmt.Fscanln(cmd.InOrStdin(), &confirm)
					if err != nil && err.Error() != "EOF" {
						// ignore error
					}
					if strings.ToLower(confirm) == "y" {
						apply = true
					} else {
						fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
						return nil
					}
				}
			}

			if apply {
				// Create Tag
				fmt.Fprintf(cmd.ErrOrStderr(), "Creating tag %s...\n", newVersion)
				if err := gitClient.Tag(cwd, newVersion); err != nil {
					return fmt.Errorf("failed to create tag: %w", err)
				}

				if push {
					fmt.Fprintln(cmd.ErrOrStderr(), "Pushing tags...")
					if err := gitClient.PushTags(cwd); err != nil {
						return fmt.Errorf("failed to push tags: %w", err)
					}
				}

				fmt.Fprintf(cmd.OutOrStdout(), "✅ Released %s successfully!\n", newVersion)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&apply, "yes", "y", false, "Automatically apply the release (tag)")
	cmd.Flags().BoolVarP(&push, "push", "p", false, "Push tags to remote after creation")
	cmd.Flags().BoolVar(&force, "force", false, "Force release without confirmation (implies --yes)")

	return cmd
}

var releaseCmd = NewReleaseCmd()

func init() {
	rootCmd.AddCommand(releaseCmd)
}
