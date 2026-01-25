package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	verifyBase   string
	verifyFile   string
	verifyStrict bool
)

var changelogVerifyCmd = &cobra.Command{
	Use:   "verify",
	Short: "Verify that the changelog has been updated",
	Long: `Checks if the changelog file has been modified relative to the base branch.
If --strict is used, it also uses AI to verify that the changelog entries match the commit history.`,
	RunE: runChangelogVerify,
}

func init() {
	changelogCmd.AddCommand(changelogVerifyCmd)
	changelogVerifyCmd.Flags().StringVar(&verifyBase, "base", "main", "Base branch to compare against")
	changelogVerifyCmd.Flags().StringVar(&verifyFile, "file", "CHANGELOG.md", "Path to changelog file")
	changelogVerifyCmd.Flags().BoolVar(&verifyStrict, "strict", false, "Use AI to verify changelog content against commits")
}

func runChangelogVerify(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	gitClient := gitClientFactory()
	if !gitClient.RepoExists(cwd) {
		return fmt.Errorf("not a git repository")
	}

	// 1. Check if file exists
	if _, err := os.Stat(verifyFile); os.IsNotExist(err) {
		return fmt.Errorf("changelog file '%s' not found", verifyFile)
	}

	// 2. Check for diffs
	// We use "git diff <base> -- <file>"
	diffOutput, err := gitClient.Run(cwd, "diff", verifyBase, "--", verifyFile)
	if err != nil {
		// git diff might fail if base doesn't exist
		return fmt.Errorf("failed to check diff (base: %s): %w", verifyBase, err)
	}

	if strings.TrimSpace(diffOutput) == "" {
		return fmt.Errorf("no changes found in %s relative to %s", verifyFile, verifyBase)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "✅ Changes detected in %s\n", verifyFile)

	if !verifyStrict {
		return nil
	}

	// 3. Strict Mode: AI Verification
	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Verifying content with AI...")

	// Get commits
	// git log base..HEAD
	rangeSpec := fmt.Sprintf("%s..HEAD", verifyBase)
	logs, err := gitClient.Log(cwd, "--pretty=format:%h %an: %s", "--no-merges", rangeSpec)
	if err != nil {
		return fmt.Errorf("failed to get git logs: %w", err)
	}
	if len(logs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "Warning: No commits found in range, but changelog changed. Suspicious but allowed.")
		return nil
	}

	// Only get added lines from diff to reduce context
	// Simple grep for lines starting with "+" but not "+++"
	var addedLines []string
	lines := strings.Split(diffOutput, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+") && !strings.HasPrefix(line, "+++") {
			addedLines = append(addedLines, strings.TrimPrefix(line, "+"))
		}
	}
	changelogContent := strings.Join(addedLines, "\n")

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-changelog-verify")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are a QA bot verifying a Changelog.
I have a list of commits and the new entries added to the Changelog.
Your job is to determine if the Changelog accurately reflects the important changes in the commits.

Commits:
%s

Changelog Additions:
%s

If the changelog is sufficient, reply with "PASS".
If there are major missing items or misrepresentations, reply with "FAIL: <explanation>".
`, strings.Join(logs, "\n"), changelogContent)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(resp)), "PASS") {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ AI Verification passed.")
		return nil
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "❌ Verification failed:\n%s\n", resp)
	return fmt.Errorf("changelog verification failed")
}
