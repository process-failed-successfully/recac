package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
)

var (
	cleanupDryRun     bool
	cleanupDays       int
	cleanupForce      bool
	cleanupMergedOnly bool
)

var gitCleanupCmd = &cobra.Command{
	Use:   "git-cleanup",
	Short: "Clean up local git branches",
	Long: `Interactive tool to clean up local git branches.
It identifies:
1. Branches that have been merged into the current branch.
2. Stale branches that haven't been touched in X days.`,
	RunE: runGitCleanup,
}

func init() {
	rootCmd.AddCommand(gitCleanupCmd)
	gitCleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Preview changes without deleting")
	gitCleanupCmd.Flags().IntVar(&cleanupDays, "days", 30, "Days threshold for stale branches")
	gitCleanupCmd.Flags().BoolVarP(&cleanupForce, "force", "f", false, "Delete without confirmation (dangerous)")
	gitCleanupCmd.Flags().BoolVar(&cleanupMergedOnly, "merged-only", false, "Only clean merged branches, ignore stale ones")
}

func runGitCleanup(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}

	client := gitClientFactory()
	if !client.RepoExists(cwd) {
		return fmt.Errorf("current directory is not a git repository")
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🔍 Analyzing branches...")

	// 1. Get Merged Branches
	mergedBranches, err := getMergedBranches(client, cwd)
	if err != nil {
		return fmt.Errorf("failed to get merged branches: %w", err)
	}

	// 2. Get Stale Branches
	var staleBranches []string
	if !cleanupMergedOnly {
		staleBranches, err = getStaleBranches(client, cwd, cleanupDays)
		if err != nil {
			return fmt.Errorf("failed to get stale branches: %w", err)
		}
	}

	// Combine and Deduplicate
	candidates := make(map[string]string) // name -> reason
	for _, b := range mergedBranches {
		candidates[b] = "Merged"
	}
	for _, b := range staleBranches {
		if _, exists := candidates[b]; !exists {
			candidates[b] = fmt.Sprintf("Stale (>%d days)", cleanupDays)
		} else {
			candidates[b] = "Merged & Stale"
		}
	}

	if len(candidates) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "✅ No cleanup candidates found.")
		return nil
	}

	// Prepare list for selection
	var options []string
	for name, reason := range candidates {
		// Format: "branch-name [Reason]"
		options = append(options, fmt.Sprintf("%s \t[%s]", name, reason))
	}

	var selectedOptions []string

	if cleanupForce {
		selectedOptions = options
	} else if !cleanupDryRun {
		// Interactive Selection
		prompt := &survey.MultiSelect{
			Message: "Select branches to delete:",
			Options: options,
		}
		err := survey.AskOne(prompt, &selectedOptions)
		if err != nil {
			return nil // Cancelled
		}
	} else {
		// Dry run without interactive: show what would happen
		// But usually dry-run just lists them.
		// Let's pretend we selected all for display purposes in dry run?
		// Or just list candidates.
		selectedOptions = options
	}

	if len(selectedOptions) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No branches selected.")
		return nil
	}

	// Execute
	count := 0
	for _, opt := range selectedOptions {
		// Extract branch name (everything before first tab or space)
		parts := strings.Split(opt, "\t")
		if len(parts) == 0 {
			continue
		}
		branch := strings.TrimSpace(parts[0])

		if cleanupDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "[Dry Run] Would delete: %s\n", branch)
		} else {
			err := client.DeleteLocalBranch(cwd, branch)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "❌ Failed to delete %s: %v\n", branch, err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "🗑️  Deleted %s\n", branch)
				count++
			}
		}
	}

	if !cleanupDryRun {
		fmt.Fprintf(cmd.OutOrStdout(), "Cleanup complete. Removed %d branches.\n", count)
	}

	return nil
}

func getMergedBranches(client IGitClient, cwd string) ([]string, error) {
	// Use for-each-ref for reliable parsing
	out, err := client.Run(cwd, "for-each-ref", "--format=%(refname:short)", "--merged=HEAD", "refs/heads/")
	if err != nil {
		return nil, err
	}

	current, err := client.CurrentBranch(cwd)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(out, "\n")
	var branches []string
	for _, line := range lines {
		b := strings.TrimSpace(line)
		if b == "" {
			continue
		}
		// Skip main/master/dev/develop protection
		if isProtected(b) {
			continue
		}
		if b == current {
			continue
		}
		branches = append(branches, b)
	}
	return branches, nil
}

func getStaleBranches(client IGitClient, cwd string, days int) ([]string, error) {
	// git for-each-ref --sort=-committerdate --format="%(committerdate:iso8601)|%(refname:short)" refs/heads/
	out, err := client.Run(cwd, "for-each-ref", "--sort=-committerdate", "--format=%(committerdate:iso8601)|%(refname:short)", "refs/heads/")
	if err != nil {
		return nil, err
	}

	current, err := client.CurrentBranch(cwd)
	if err != nil {
		return nil, err
	}

	threshold := time.Now().AddDate(0, 0, -days)
	var branches []string
	lines := strings.Split(out, "\n")
	for _, line := range lines {
		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			continue
		}
		dateStr := strings.TrimSpace(parts[0])
		name := strings.TrimSpace(parts[1])

		if name == "" || name == current || isProtected(name) {
			continue
		}

		// Parse date (ISO8601)
		// git output might be: 2023-10-26 14:00:00 +0000
		t, err := time.Parse("2006-01-02 15:04:05 -0700", dateStr)
		if err != nil {
			// Try RFC3339 just in case
			t, err = time.Parse(time.RFC3339, dateStr)
			if err != nil {
				continue // skip if can't parse
			}
		}

		if t.Before(threshold) {
			branches = append(branches, name)
		}
	}
	return branches, nil
}

func isProtected(branch string) bool {
	protected := []string{"main", "master", "dev", "develop", "staging", "production"}
	for _, p := range protected {
		if branch == p {
			return true
		}
	}
	return false
}
