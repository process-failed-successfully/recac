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
	prBase      string
	prCreate    bool
	prDraft     bool
	prTitleOnly bool
	prBodyOnly  bool
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Generate (and optionally create) a Pull Request",
	Long: `Generates a Pull Request title and description based on the commits and diff
between the current branch and the base branch using AI.

Can optionally create the PR using the 'gh' CLI tool.`,
	RunE: runPR,
}

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.Flags().StringVarP(&prBase, "base", "b", "main", "Base branch to target")
	prCmd.Flags().BoolVarP(&prCreate, "create", "c", false, "Create the PR using 'gh' CLI")
	prCmd.Flags().BoolVar(&prDraft, "draft", false, "Create as draft PR (requires --create)")
	prCmd.Flags().BoolVar(&prTitleOnly, "title-only", false, "Output only the generated title")
	prCmd.Flags().BoolVar(&prBodyOnly, "body-only", false, "Output only the generated body")
}

func runPR(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	gitClient := gitClientFactory()
	if !gitClient.RepoExists(cwd) {
		return fmt.Errorf("not a git repository")
	}

	// 1. Determine Current Branch
	currentBranch, err := gitClient.CurrentBranch(cwd)
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}

	if currentBranch == prBase {
		return fmt.Errorf("you are already on the base branch '%s'", prBase)
	}

	// 2. Fetch Commits
	// git log base..HEAD
	logRange := fmt.Sprintf("%s..HEAD", prBase)
	logs, err := gitClient.Log(cwd, "--pretty=format:%h %an: %s", "--no-merges", logRange)
	if err != nil {
		return fmt.Errorf("failed to get git logs (ensure '%s' exists): %w", prBase, err)
	}

	if len(logs) == 0 {
		return fmt.Errorf("no commits found between %s and HEAD", prBase)
	}

	// 3. Fetch Diff
	// git diff base...HEAD (triple dot for merge base comparison is usually safer for PRs)
	// But gitClient.Diff takes two commits.
	// Let's use base..HEAD (double dot) which is standard diff.
	// Actually for PRs, we usually want what's in the feature branch.
	// If interface only supports Diff(a, b), it does `git diff a b`.
	// `git diff main feature`
	diff, err := gitClient.Diff(cwd, prBase, "HEAD")
	if err != nil {
		// Try to fetch origin first?
		// warning but proceed
		fmt.Fprintf(cmd.ErrOrStderr(), "Warning: failed to get diff: %v\n", err)
	}

	// Truncate diff
	if len(diff) > 20000 {
		// Find the last newline before the limit to avoid cutting lines or runes
		cutIndex := strings.LastIndex(diff[:20000], "\n")
		if cutIndex == -1 {
			cutIndex = 20000
		}
		diff = diff[:cutIndex] + "\n... (truncated)"
	}

	// 4. Generate Content
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-pr")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Analyzing %d commits and diff...\n", len(logs))

	prompt := fmt.Sprintf(`You are an expert developer creating a Pull Request.
Source Branch: %s
Target Branch: %s

Commits:
%s

Diff Summary:
%s

Task:
Generate a clear and concise PR Title and Description.
- The Title should follow Conventional Commits if possible.
- The Description should explain the changes and the reasoning.

Output Format:
TITLE: <title>
BODY:
<body>
`, currentBranch, prBase, strings.Join(logs, "\n"), diff)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	// Parse Response
	resp = utils.CleanCodeBlock(resp)
	var title, body string

	lines := strings.Split(resp, "\n")
	var bodyLines []string
	inBody := false

	for _, line := range lines {
		if strings.HasPrefix(line, "TITLE:") {
			title = strings.TrimSpace(strings.TrimPrefix(line, "TITLE:"))
			inBody = false
		} else if strings.HasPrefix(line, "BODY:") {
			inBody = true
			// content might start on same line
			rest := strings.TrimPrefix(line, "BODY:")
			if strings.TrimSpace(rest) != "" {
				bodyLines = append(bodyLines, strings.TrimSpace(rest))
			}
		} else if inBody {
			bodyLines = append(bodyLines, line)
		}
	}
	body = strings.TrimSpace(strings.Join(bodyLines, "\n"))

	// Fallback parsing if format wasn't respected
	if title == "" {
		// Assume first line is title
		if len(lines) > 0 {
			title = lines[0]
			if len(lines) > 1 {
				body = strings.Join(lines[1:], "\n")
			}
		}
	}

	// Output
	if !prCreate {
		if !prBodyOnly {
			if prTitleOnly {
				fmt.Fprintln(cmd.OutOrStdout(), title)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Title:", title)
			}
		}
		if !prTitleOnly {
			if prBodyOnly {
				fmt.Fprintln(cmd.OutOrStdout(), body)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "\nBody:\n", body)
			}
		}
	}

	// 5. Create PR via gh
	if prCreate {
		if title == "" {
			return fmt.Errorf("failed to generate PR title")
		}

		ghArgs := []string{"pr", "create", "--base", prBase, "--head", currentBranch, "--title", title, "--body", body}
		if prDraft {
			ghArgs = append(ghArgs, "--draft")
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Creating PR on %s...\n", prBase)

		c := execCommand("gh", ghArgs...)
		c.Stdout = cmd.OutOrStdout()
		c.Stderr = cmd.ErrOrStderr()

		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to run 'gh': %w (is it installed?)", err)
		}
	}

	return nil
}
