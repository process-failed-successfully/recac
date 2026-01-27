package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	prBase   string
	prCreate bool
	prDraft  bool
	prTitle  string
)

var prCmd = &cobra.Command{
	Use:   "pr",
	Short: "Generate Pull Request description and optional PR",
	Long:  `Generates a Pull Request title and description based on the git diff between the current branch and the base branch (default: main). Optionally creates the PR using the 'gh' CLI.`,
	RunE:  runPr,
}

func init() {
	rootCmd.AddCommand(prCmd)
	prCmd.Flags().StringVarP(&prBase, "base", "b", "main", "Base branch to compare against")
	prCmd.Flags().BoolVarP(&prCreate, "create", "c", false, "Create the PR using 'gh' CLI")
	prCmd.Flags().BoolVar(&prDraft, "draft", false, "Create as draft PR (only with --create)")
	prCmd.Flags().StringVarP(&prTitle, "title", "t", "", "Override generated title")
}

func runPr(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}

	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	// 1. Get Git Client
	// We use the execCommand wrapper or gitClientFactory.
	// existing commands mix them. Let's use execCommand for simplicity in specific git operations not in generic client,
	// or stick to gitClient if it has what we need.
	// gitClient has Diff but maybe not "diff base...head".
	// Let's use execCommand for specific git logic here to match the pattern in `commit.go` (it uses gitClient)
	// But `commit.go` uses `DiffStaged`. We need diff against another branch.
	// Let's stick to execCommand for "git diff base...HEAD" which is safest for PRs.

	// Check if we are in a git repo
	if err := execCommand("git", "rev-parse", "--is-inside-work-tree").Run(); err != nil {
		return fmt.Errorf("not a git repository")
	}

	// Get current branch
	out, err := execCommand("git", "branch", "--show-current").Output()
	if err != nil {
		return fmt.Errorf("failed to get current branch: %w", err)
	}
	currentBranch := strings.TrimSpace(string(out))

	if currentBranch == "" {
		return fmt.Errorf("detached HEAD state, please checkout a branch")
	}

	if currentBranch == prBase {
		return fmt.Errorf("current branch is same as base branch (%s)", prBase)
	}

	// Get Diff
	// We use 3 dots ... to get changes since divergence
	diffCmd := execCommand("git", "diff", fmt.Sprintf("%s...%s", prBase, currentBranch))
	diffOut, err := diffCmd.Output()
	if err != nil {
		// Fallback to 2 dots if 3 dots fail (e.g. shallow clone issue?)
		diffCmd = execCommand("git", "diff", fmt.Sprintf("%s..%s", prBase, currentBranch))
		diffOut, err = diffCmd.Output()
		if err != nil {
			return fmt.Errorf("failed to get git diff: %w", err)
		}
	}

	diffStr := string(diffOut)
	if strings.TrimSpace(diffStr) == "" {
		return fmt.Errorf("no changes detected between %s and %s", prBase, currentBranch)
	}

	// 2. Generate Description
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-pr")
	if err != nil {
		return fmt.Errorf("failed to create agent: %w", err)
	}

	fmt.Fprintf(cmd.ErrOrStderr(), "Generating PR description for branch '%s' against '%s'...\n", currentBranch, prBase)

	prompt := fmt.Sprintf(`You are an expert developer.
Generate a Pull Request Title and Description for the following changes.
The output format must be:
TITLE: <Concise Title>
DESCRIPTION:
<Markdown Description>

Include sections for:
- Summary
- Key Changes
- Testing Instructions

Changes:
'''
%s
'''`, diffStr)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return fmt.Errorf("agent failed: %w", err)
	}

	resp = utils.CleanCodeBlock(resp)

	// Parse Response
	var generatedTitle, generatedBody string
	lines := strings.Split(resp, "\n")
	var bodyBuilder strings.Builder
	inBody := false

	for _, line := range lines {
		if strings.HasPrefix(line, "TITLE:") {
			generatedTitle = strings.TrimSpace(strings.TrimPrefix(line, "TITLE:"))
		} else if strings.HasPrefix(line, "DESCRIPTION:") {
			inBody = true
		} else if inBody {
			bodyBuilder.WriteString(line + "\n")
		} else {
			// If format is loose, treat everything as body if title is found, or first line as title?
			// Let's assume strict format for now, or fallback.
			if generatedTitle == "" && len(line) > 0 {
				generatedTitle = line
				inBody = true // Assume rest is body
			} else if inBody {
				bodyBuilder.WriteString(line + "\n")
			}
		}
	}
	generatedBody = strings.TrimSpace(bodyBuilder.String())

	// Override title if flag provided
	if prTitle != "" {
		generatedTitle = prTitle
	}

	// Output
	fmt.Fprintln(cmd.OutOrStdout(), "========================================")
	fmt.Fprintf(cmd.OutOrStdout(), "Title: %s\n", generatedTitle)
	fmt.Fprintln(cmd.OutOrStdout(), "Description:")
	fmt.Fprintln(cmd.OutOrStdout(), generatedBody)
	fmt.Fprintln(cmd.OutOrStdout(), "========================================")

	// 3. Create PR if requested
	if prCreate {
		// Check for gh cli
		if _, err := lookPathFunc("gh"); err != nil {
			return fmt.Errorf("'gh' CLI not found. Please install it to use --create")
		}

		// Write body to temp file
		tmpFile, err := os.CreateTemp("", "recac-pr-body-*.md")
		if err != nil {
			return fmt.Errorf("failed to create temp file: %w", err)
		}
		defer os.Remove(tmpFile.Name())
		if _, err := tmpFile.WriteString(generatedBody); err != nil {
			return err
		}
		tmpFile.Close()

		ghArgs := []string{"pr", "create", "--title", generatedTitle, "--body-file", tmpFile.Name(), "--base", prBase}
		if prDraft {
			ghArgs = append(ghArgs, "--draft")
		}

		fmt.Fprintf(cmd.ErrOrStderr(), "Creating PR on GitHub...\n")
		ghCmd := execCommand("gh", ghArgs...)
		ghCmd.Stdout = cmd.OutOrStdout()
		ghCmd.Stderr = cmd.ErrOrStderr()
		if err := ghCmd.Run(); err != nil {
			return fmt.Errorf("failed to create PR: %w", err)
		}
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\nTip: Run with --create to automatically create the PR on GitHub.")
	}

	return nil
}
