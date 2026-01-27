package main

import (
	"encoding/json"
	"fmt"
	"os"
	"recac/internal/utils"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	prBaseBranch string
	prCreate     bool
	prDryRun     bool
	prTitle      string
	prBody       string
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Generate and create a Pull Request",
	Long: `Analyze the git diff between the current branch and a base branch to generate
a PR title and description using AI. Optionally creates the PR using GitHub CLI (gh).`,
	RunE: runPR,
}

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.Flags().StringVarP(&prBaseBranch, "base", "b", "main", "Base branch to merge into")
	prCmd.Flags().BoolVar(&prCreate, "create", false, "Create the PR using 'gh' CLI")
	prCmd.Flags().BoolVar(&prDryRun, "dry-run", false, "Just print the generated content without creating")
	prCmd.Flags().StringVarP(&prTitle, "title", "t", "", "Override PR title")
	prCmd.Flags().StringVarP(&prBody, "description", "d", "", "Override PR description")
}

func runPR(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	git := gitClientFactory()

	// 1. Validate Repository
	if !git.RepoExists(cwd) {
		return fmt.Errorf("current directory is not a git repository")
	}

	// 2. Get Current Branch
	currentBranch, err := git.CurrentBranch(cwd)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	if currentBranch == "" {
		return fmt.Errorf("detached HEAD or no branch")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "🔍 Analyzing changes between %s and %s...\n", prBaseBranch, currentBranch)

	// 3. Get Diff
	diff, err := git.Diff(cwd, prBaseBranch, currentBranch)
	if err != nil {
		return fmt.Errorf("failed to get diff: %w", err)
	}
	if strings.TrimSpace(diff) == "" {
		return fmt.Errorf("no changes detected between %s and %s", prBaseBranch, currentBranch)
	}

	// 4. Generate Content (if not provided)
	title := prTitle
	body := prBody

	if title == "" || body == "" {
		// Prepare Prompt
		provider := viper.GetString("provider")
		model := viper.GetString("model")

		ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-pr")
		if err != nil {
			return fmt.Errorf("failed to initialize agent: %w", err)
		}

		prompt := fmt.Sprintf(`You are an expert software engineer.
Generate a concise and descriptive Pull Request Title and Description for the following git diff.

Diff:
'''
%s
'''

Return the result as a raw JSON object with "title" and "description" keys.
Do not wrap in markdown code blocks.`, diff)

		fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating PR description...")
		resp, err := ag.Send(ctx, prompt)
		if err != nil {
			return fmt.Errorf("agent failed: %w", err)
		}

		// Parse Response
		jsonStr := utils.CleanJSONBlock(resp)
		var result map[string]string
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			// Fallback if JSON parsing fails
			fmt.Fprintf(cmd.ErrOrStderr(), "Warning: Failed to parse JSON response: %v\n", err)
			if title == "" {
				title = "Update"
			}
			if body == "" {
				body = resp
			}
		} else {
			if title == "" {
				title = result["title"]
			}
			if body == "" {
				body = result["description"]
			}
		}
	}

	// 5. Output / Action
	fmt.Fprintln(cmd.OutOrStdout(), "\n📋 Proposed PR Content:")
	fmt.Fprintf(cmd.OutOrStdout(), "Title: %s\n", title)
	fmt.Fprintf(cmd.OutOrStdout(), "Description:\n%s\n", body)
	fmt.Fprintln(cmd.OutOrStdout(), "")

	if prDryRun {
		return nil
	}

	if prCreate {
		fmt.Fprintln(cmd.OutOrStdout(), "🚀 Creating PR...")
		url, err := git.CreatePR(cwd, title, body, prBaseBranch)
		if err != nil {
			return fmt.Errorf("failed to create PR: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ PR Created: %s\n", url)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "To create this PR, run:")
		// Escape quotes for shell safety
		safeTitle := strings.ReplaceAll(title, "\"", "\\\"")
		safeBody := strings.ReplaceAll(body, "\"", "\\\"")
		fmt.Fprintf(cmd.OutOrStdout(), "gh pr create --base %s --title \"%s\" --body \"%s\"\n", prBaseBranch, safeTitle, safeBody)
		fmt.Fprintln(cmd.OutOrStdout(), "\nOr use --create flag to automate this.")
	}

	return nil
}
