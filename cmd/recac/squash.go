package main

import (
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	squashBaseBranch string
	squashApply      bool
	squashForce      bool
)

var squashCmd = &cobra.Command{
	Use:   "squash",
	Short: "Squash recent commits into a single commit with an AI-generated message",
	Long: `Analyze the commits and diffs since a base branch (or specified commit),
generate a Conventional Commit message using AI, and optionally squash them
into a single commit.

This tool helps keep your git history clean by combining messy work-in-progress
commits into one logical commit.`,
	RunE: runSquash,
}

func init() {
	rootCmd.AddCommand(squashCmd)
	squashCmd.Flags().StringVarP(&squashBaseBranch, "base", "b", "main", "Base branch or commit to squash down to")
	squashCmd.Flags().BoolVarP(&squashApply, "yes", "y", false, "Automatically apply the squash (reset soft + commit)")
	squashCmd.Flags().BoolVar(&squashForce, "force", false, "Force squash without confirmation (implies --yes)")
}

func runSquash(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	git := gitClientFactory()

	// 1. Validate Repository
	if !git.RepoExists(cwd) {
		return fmt.Errorf("not a git repository")
	}

	// 2. Find Merge Base
	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Finding commits since %s...\n", squashBaseBranch)
	mergeBase, err := git.MergeBase(cwd, squashBaseBranch, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to find merge base with %s: %w", squashBaseBranch, err)
	}
	if mergeBase == "" {
		return fmt.Errorf("no merge base found")
	}

	// 3. Get Commits to Squash
	logArgs := []string{"--pretty=format:%h %an: %s", "--no-merges", fmt.Sprintf("%s..HEAD", mergeBase)}
	logs, err := git.Log(cwd, logArgs...)
	if err != nil {
		return fmt.Errorf("failed to get git logs: %w", err)
	}

	if len(logs) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No commits found to squash.")
		return nil
	}

	// 4. Get Diff
	diff, err := git.Diff(cwd, mergeBase, "HEAD")
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}

	if strings.TrimSpace(diff) == "" {
		return fmt.Errorf("no changes detected between %s and HEAD", mergeBase)
	}

	// 5. Generate AI Message
	fmt.Fprintf(cmd.OutOrStdout(), "🤖 Analyzing %d commits and diffs to generate a squash message...\n", len(logs))

	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-squash")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are an expert software engineer.
I need to squash the following commits into a single commit.
Generate a concise and descriptive Conventional Commit message (subject line and optional body) that summarizes all these changes.

Commits:
%s

Diff:
'''
%s
'''

Output ONLY the commit message. Do not include markdown formatting (like '''text...''').`, strings.Join(logs, "\n"), diff)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	msg := utils.CleanCodeBlock(resp)

	fmt.Fprintln(cmd.OutOrStdout(), "\n📋 Proposed Squash Commit Message:")
	fmt.Fprintln(cmd.OutOrStdout(), "------------------------------------------------")
	fmt.Fprintln(cmd.OutOrStdout(), msg)
	fmt.Fprintln(cmd.OutOrStdout(), "------------------------------------------------")
	fmt.Fprintln(cmd.OutOrStdout())

	// 6. Confirmation / Execution
	apply := squashApply
	if !apply {
		if squashForce {
			apply = true
		} else {
			fmt.Fprint(cmd.OutOrStdout(), "Proceed with this squash? [y/N]: ")
			var confirm string
			// Handle non-interactive environments cleanly
			_, err := fmt.Fscanln(cmd.InOrStdin(), &confirm)
			if err != nil && err.Error() != "EOF" {
				// ignore scan errors from empty buffers
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
		fmt.Fprintln(cmd.OutOrStdout(), "🚀 Squashing commits...")

		// Reset soft to merge base
		if err := git.ResetSoft(cwd, mergeBase); err != nil {
			return fmt.Errorf("failed to reset soft: %w", err)
		}

		// Commit with new message
		if err := git.Commit(cwd, msg); err != nil {
			return fmt.Errorf("failed to commit squashed changes: %w", err)
		}

		fmt.Fprintln(cmd.OutOrStdout(), "✅ Commits squashed successfully!")
	}

	return nil
}
