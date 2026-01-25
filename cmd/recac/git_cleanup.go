package main

import (
	"fmt"
	"os"
	"strings"
	"time"

	"recac/internal/ui"

	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

var (
	gitCleanupBase string
	gitCleanupDays int
)

var gitCleanupCmd = &cobra.Command{
	Use:   "git-cleanup",
	Short: "Interactively clean up local branches",
	Long:  `Scan local branches, identify those merged into the base branch, and allow interactive deletion.`,
	RunE:  runGitCleanup,
}

func init() {
	rootCmd.AddCommand(gitCleanupCmd)
	gitCleanupCmd.Flags().StringVar(&gitCleanupBase, "base", "main", "Base branch to check merge status against")
	gitCleanupCmd.Flags().IntVar(&gitCleanupDays, "days", 30, "Days threshold for stale branches")
}

func runGitCleanup(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	gitClient := gitClientFactory()
	if !gitClient.RepoExists(cwd) {
		return fmt.Errorf("not a git repository")
	}

	// 1. Get all local branches with details
	// Format: refname:short|committerdate:iso8601-strict|authorname
	out, err := gitClient.Run(cwd, "for-each-ref", "--format=%(refname:short)|%(committerdate:iso8601-strict)|%(authorname)", "refs/heads")
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	lines := strings.Split(out, "\n")
	var branches []ui.BranchItem

	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) < 3 {
			continue
		}
		name := parts[0]
		dateStr := parts[1]
		author := parts[2]

		// Skip current branch to avoid deleting it accidentally (safety)
		current, _ := gitClient.CurrentBranch(cwd)
		if name == current {
			continue
		}
		// Skip base branch
		if name == gitCleanupBase {
			continue
		}

		item := ui.BranchItem{
			Name:   name,
			Author: author,
			Status: ui.StatusActive,
		}

		// Parse date
		t, err := time.Parse(time.RFC3339, dateStr)
		if err == nil {
			item.LastCommit = t.Format("2006-01-02")
			if time.Since(t).Hours() > float64(gitCleanupDays*24) {
				item.Status = ui.StatusStale
			}
		}

		branches = append(branches, item)
		// Store pointer to update status later
		// We can't easily store pointer to slice element as realloc invalidates it.
		// So we'll map index.
	}

	// 2. Identify Merged Branches
	// git branch --merged base
	mergedOut, err := gitClient.Run(cwd, "branch", "--merged", gitCleanupBase)
	if err == nil {
		mergedLines := strings.Split(mergedOut, "\n")
		mergedSet := make(map[string]bool)
		for _, line := range mergedLines {
			mergedSet[strings.TrimSpace(line)] = true
		}

		for i := range branches {
			if mergedSet[branches[i].Name] {
				branches[i].Status = ui.StatusMerged
			}
		}
	} else {
		// If base branch doesn't exist, we might fail. Warn user?
		fmt.Fprintf(cmd.OutOrStderr(), "Warning: Could not check merged status against '%s': %v\n", gitCleanupBase, err)
	}

	if len(branches) == 0 {
		cmd.Println("No other local branches found.")
		return nil
	}

	// 3. Start TUI
	m := ui.NewGitCleanupModel(branches)
	p := tea.NewProgram(m, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	cleanupModel, ok := finalModel.(ui.GitCleanupModel)
	if !ok {
		return fmt.Errorf("internal error: invalid model returned")
	}

	selected := cleanupModel.GetSelectedBranches()
	if len(selected) == 0 {
		cmd.Println("No branches selected for deletion.")
		return nil
	}

	// 4. Delete Branches
	cmd.Printf("Deleting %d branches...\n", len(selected))
	successCount := 0
	failCount := 0

	for _, branch := range selected {
		// Use -D (force delete) because we already confirmed in TUI and checked merged status visually
		err := gitClient.DeleteLocalBranch(cwd, branch)
		if err != nil {
			cmd.Printf("Failed to delete %s: %v\n", branch, err)
			failCount++
		} else {
			cmd.Printf("Deleted %s\n", branch)
			successCount++
		}
	}

	cmd.Printf("Done. Deleted: %d, Failed: %d\n", successCount, failCount)
	return nil
}
