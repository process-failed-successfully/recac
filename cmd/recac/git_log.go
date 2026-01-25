package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"recac/internal/ui"

	"github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var gitLogCmd = &cobra.Command{
	Use:   "git-log",
	Short: "Interactive Git Log Explorer",
	Long:  `Browse git commit history, view diffs, and ask AI to explain or audit commits using a TUI.`,
	RunE:  runGitLog,
}

func init() {
	rootCmd.AddCommand(gitLogCmd)
}

func runGitLog(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	gitClient := gitClientFactory()
	if !gitClient.RepoExists(cwd) {
		return fmt.Errorf("not a git repository")
	}

	// 1. Fetch Logs
	// Format: Hash|Author|Date|Message
	logLines, err := gitClient.Log(cwd, "--pretty=format:%h|%an|%ad|%s", "-n", "100")
	if err != nil {
		return fmt.Errorf("failed to fetch git log: %w", err)
	}

	var commits []ui.CommitItem
	for _, line := range logLines {
		parts := strings.SplitN(line, "|", 4)
		if len(parts) < 4 {
			continue
		}
		commits = append(commits, ui.CommitItem{
			Hash:    parts[0],
			Author:  parts[1],
			Date:    parts[2],
			Message: parts[3],
		})
	}

	if len(commits) == 0 {
		return fmt.Errorf("no commits found")
	}

	// 2. Define Callbacks
	fetchDiff := func(hash string) (string, error) {
		diff, err := gitClient.Diff(cwd, hash+"^", hash)
		if err != nil {
			return "", fmt.Errorf("failed to fetch diff (note: root commit diffs may not be supported): %w", err)
		}
		return diff, nil
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	// Helper for AI analysis
	analyzeCommit := func(hash, promptPrefix string) (string, error) {
		diff, err := fetchDiff(hash)
		if err != nil {
			return "", fmt.Errorf("failed to get diff: %w", err)
		}

		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-git-log")
		if err != nil {
			return "", fmt.Errorf("failed to create agent: %w", err)
		}

		prompt := fmt.Sprintf("%s\n\nCommit Hash: %s\n\nDiff:\n```\n%s\n```", promptPrefix, hash, diff)
		return ag.Send(ctx, prompt)
	}

	explainFunc := func(hash string) (string, error) {
		return analyzeCommit(hash, "Please explain this commit in plain English. Describe the changes and their impact.")
	}

	auditFunc := func(hash string) (string, error) {
		return analyzeCommit(hash, "Please audit this commit for security vulnerabilities, bugs, or bad practices. Be concise.")
	}

	// 3. Start TUI
	m := ui.NewGitLogModel(commits, fetchDiff, explainFunc, auditFunc)
	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running TUI: %w", err)
	}

	return nil
}
