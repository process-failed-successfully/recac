package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	stashAnalyze bool
)

var stashCmd = &cobra.Command{
	Use:   "stash",
	Short: "Manage git stashes with AI superpowers",
	Long:  `Manage git stashes, including smart saving with AI-generated messages and analyzed listing.`,
}

var stashSaveCmd = &cobra.Command{
	Use:   "save [message]",
	Short: "Save local changes to a stash",
	Long:  `Save local changes (including untracked files) to a new stash. If no message is provided, AI will generate one based on the diff.`,
	RunE:  runStashSave,
}

var stashListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all stashes",
	Long:  `List all stashes. Use --analyze to get AI-generated summaries of what's inside each stash.`,
	RunE:  runStashList,
}

var stashPopCmd = &cobra.Command{
	Use:   "pop [index]",
	Short: "Pop a stash",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStashAction(cmd, args, "pop")
	},
}

var stashApplyCmd = &cobra.Command{
	Use:   "apply [index]",
	Short: "Apply a stash",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStashAction(cmd, args, "apply")
	},
}

var stashDropCmd = &cobra.Command{
	Use:   "drop [index]",
	Short: "Drop a stash",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStashAction(cmd, args, "drop")
	},
}

var stashShowCmd = &cobra.Command{
	Use:   "show [index]",
	Short: "Show the diff of a stash",
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runStashAction(cmd, args, "show")
	},
}

var stashClearCmd = &cobra.Command{
	Use:   "clear",
	Short: "Clear all stashes",
	RunE:  runStashClear,
}

func init() {
	rootCmd.AddCommand(stashCmd)
	stashCmd.AddCommand(stashSaveCmd)
	stashCmd.AddCommand(stashListCmd)
	stashCmd.AddCommand(stashPopCmd)
	stashCmd.AddCommand(stashApplyCmd)
	stashCmd.AddCommand(stashDropCmd)
	stashCmd.AddCommand(stashShowCmd)
	stashCmd.AddCommand(stashClearCmd)

	stashListCmd.Flags().BoolVarP(&stashAnalyze, "analyze", "a", false, "Analyze stash contents with AI")
}

func runStashSave(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	git := gitClientFactory()

	message := ""
	if len(args) > 0 {
		message = strings.Join(args, " ")
	} else {
		// AI Generation
		diff, err := git.DiffStaged(cwd)
		if err != nil || strings.TrimSpace(diff) == "" {
			// Try diffing against HEAD (unstaged)
			d, e := git.Run(cwd, "diff", "HEAD")
			if e == nil {
				diff = d
			}
		}

		if strings.TrimSpace(diff) != "" {
			fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating stash message...")
			ctx := context.Background()
			provider := viper.GetString("provider")
			model := viper.GetString("model")
			ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-stash")
			if err != nil {
				return err
			}

			prompt := fmt.Sprintf("Generate a very concise (max 10 words) git stash message for these changes:\n\n%s", diff)
			msg, err := ag.Send(ctx, prompt)
			if err != nil {
				return err
			}
			message = strings.TrimSpace(msg)
			message = strings.Trim(message, "\"`")
			fmt.Fprintf(cmd.OutOrStdout(), "Generated: %s\n", message)
		} else {
			message = "WIP (Untracked files)"
		}
	}

	return git.StashPush(cwd, message)
}

func runStashList(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	git := gitClientFactory()

	lines, err := git.StashList(cwd)
	if err != nil {
		return err
	}

	if len(lines) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No stashes found.")
		return nil
	}

	if !stashAnalyze {
		for _, line := range lines {
			fmt.Fprintln(cmd.OutOrStdout(), line)
		}
		return nil
	}

	// Analyze
	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Analyzing stashes (top 5)...")
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-stash-analyze")
	if err != nil {
		return err
	}

	count := 0
	for _, line := range lines {
		if count >= 5 {
			break
		}
		parts := strings.SplitN(line, ":", 2)
		id := parts[0] // stash@{0}

		diff, err := git.StashShow(cwd, id)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to show stash %s: %v\n", id, err)
			continue
		}

		summary := "No changes detected"
		if strings.TrimSpace(diff) != "" {
			// Truncate diff
			if len(diff) > 2000 {
				diff = diff[:2000] + "\n...(truncated)"
			}
			prompt := fmt.Sprintf("Summarize this git stash content in 1 sentence:\n\n%s", diff)
			s, err := ag.Send(ctx, prompt)
			if err == nil {
				summary = strings.TrimSpace(s)
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", id, summary)
		count++
	}

	return nil
}

func runStashAction(cmd *cobra.Command, args []string, action string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	git := gitClientFactory()

	id := ""
	if len(args) > 0 {
		id = args[0]
	}

	switch action {
	case "pop":
		// Use Run to support ID if needed, and to capture output to print
		runArgs := []string{"stash", "pop"}
		if id != "" {
			runArgs = append(runArgs, id)
		}
		out, err := git.Run(cwd, runArgs...)
		if out != "" {
			fmt.Fprintln(cmd.OutOrStdout(), out)
		}
		return err

	case "apply":
		return git.StashApply(cwd, id)
	case "drop":
		return git.StashDrop(cwd, id)
	case "show":
		out, err := git.StashShow(cwd, id)
		if err == nil {
			fmt.Fprintln(cmd.OutOrStdout(), out)
		}
		return err
	}
	return nil
}

func runStashClear(cmd *cobra.Command, args []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	git := gitClientFactory()
	return git.StashClear(cwd)
}
