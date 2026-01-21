package main

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"recac/internal/agent"
	"recac/internal/utils"

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	resolveAuto bool
	resolveFile string
)

var resolveCmd = &cobra.Command{
	Use:   "resolve",
	Short: "Resolve git merge conflicts using AI",
	Long: `Scans for files with merge conflicts (standard <<<<<<< markers) and uses the AI agent to propose a resolution.
It intelligently merges the changes by understanding the code context.`,
	RunE: runResolve,
}

func init() {
	rootCmd.AddCommand(resolveCmd)
	resolveCmd.Flags().BoolVar(&resolveAuto, "auto", false, "Automatically accept high-confidence resolutions without prompting")
	resolveCmd.Flags().StringVar(&resolveFile, "file", "", "Resolve a specific file (optional)")
}

func runResolve(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	var files []string

	if resolveFile != "" {
		files = []string{resolveFile}
	} else {
		// Auto-detect conflicted files
		fmt.Fprintln(cmd.OutOrStdout(), "🔍 Scanning for conflicts...")
		detected, err := getConflictedFiles(cwd)
		if err != nil {
			// Fallback: git diff --check or similar might be noisy.
			// Try grep?
			return fmt.Errorf("failed to detect conflicts: %w", err)
		}
		if len(detected) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ No conflicted files found.")
			return nil
		}
		files = detected
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d conflicted files.\n", len(files))

	// Initialize Agent
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-resolve")
	if err != nil {
		return fmt.Errorf("failed to initialize agent: %w", err)
	}

	for i, file := range files {
		fmt.Fprintf(cmd.OutOrStdout(), "\n[%d/%d] Resolving %s...\n", i+1, len(files), file)

		if err := resolveOneFile(ctx, cmd, ag, file, resolveAuto); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "❌ Failed to resolve %s: %v\n", file, err)
			continue
		}
	}

	return nil
}

func getConflictedFiles(cwd string) ([]string, error) {
	// Method 1: git diff --name-only --diff-filter=U
	cmd := execCommand("git", "diff", "--name-only", "--diff-filter=U")
	out, err := cmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var result []string
		for _, l := range lines {
			if l != "" {
				result = append(result, l)
			}
		}
		if len(result) > 0 {
			return result, nil
		}
	}

	// Method 2: grep recursive for markers (if not a git repo or git is confused)
	// grep -r -l "<<<<<<<" .
	grepCmd := execCommand("grep", "-r", "-l", "<<<<<<<", ".")
	out, err = grepCmd.Output()
	if err == nil {
		lines := strings.Split(strings.TrimSpace(string(out)), "\n")
		var result []string
		for _, l := range lines {
			if l != "" {
				result = append(result, l)
			}
		}
		return result, nil
	}

	return []string{}, nil
}

func resolveOneFile(ctx context.Context, cmd *cobra.Command, ag agent.Agent, filePath string, auto bool) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}

	fileContent := string(content)

	// Naive regex for conflicts
	// <<<<<<< HEAD
	// ...
	// =======
	// ...
	// >>>>>>> feature
	conflictRe := regexp.MustCompile(`(?s)<<<<<<< (.*?)\n(.*?)\n=======\n(.*?)\n>>>>>>> (.*?)\n`)
	matches := conflictRe.FindAllStringSubmatch(fileContent, -1)

	if len(matches) == 0 {
		return fmt.Errorf("no standard conflict markers found in %s", filePath)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Found %d conflict(s) in file.\n", len(matches))

	// For simplicity, we resolve the WHOLE file at once if conflicts are close,
	// or we iterate. Iterating allows granular control.
	// But giving the agent the whole file context is better.
	// Let's replace conflicts with placeholders or just ask agent to return the FIXED file content?
	// Asking for full file content is token-heavy but safer for context.

	prompt := fmt.Sprintf(`You are an expert Git Merge Resolver.
The following file contains %d merge conflict(s).
Your task is to resolve them intelligently.

Rules:
1. Preserve the intent of both changes if possible.
2. If changes conflict directly, choose the most logical, bug-free, and consistent implementation.
3. Remove the conflict markers (<<<<<<<, =======, >>>>>>>).
4. Return ONLY the fully resolved content of the file. Do not use markdown blocks.

File: %s
Content:
%s
`, len(matches), filePath, fileContent)

	fmt.Fprintln(cmd.OutOrStdout(), "🤖 Generating resolution...")

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return err
	}

	resolvedContent := utils.CleanCodeBlock(resp)

	// Verification: Check if markers remain
	if strings.Contains(resolvedContent, "<<<<<<<") || strings.Contains(resolvedContent, ">>>>>>>") {
		return fmt.Errorf("agent failed to remove all conflict markers")
	}

	if auto {
		return os.WriteFile(filePath, []byte(resolvedContent), 0644)
	}

	// Interactive review
	fmt.Fprintln(cmd.OutOrStdout(), "\n--- Proposed Resolution (Preview) ---")
	// Show a truncated preview or diff?
	// Showing diff is hard without writing it first.
	// Let's show the first few lines of the resolved content or statistics.
	lines := strings.Split(resolvedContent, "\n")
	previewLines := 10
	if len(lines) < previewLines {
		previewLines = len(lines)
	}
	fmt.Fprintln(cmd.OutOrStdout(), strings.Join(lines[:previewLines], "\n"))
	if len(lines) > previewLines {
		fmt.Fprintf(cmd.OutOrStdout(), "... (%d more lines)\n", len(lines)-previewLines)
	}
	fmt.Fprintln(cmd.OutOrStdout(), "-------------------------------------")

	action := ""
	err = askOneFunc(&survey.Select{
		Message: fmt.Sprintf("Accept resolution for %s?", filePath),
		Options: []string{"Accept", "Skip", "Abort"},
	}, &action)
	if err != nil {
		return err
	}

	switch action {
	case "Accept":
		if err := os.WriteFile(filePath, []byte(resolvedContent), 0644); err != nil {
			return err
		}
		fmt.Fprintf(cmd.OutOrStdout(), "✅ Resolved %s\n", filePath)
		// Optional: git add
		add := false
		askOneFunc(&survey.Confirm{Message: "Stage this file (git add)?", Default: true}, &add)
		if add {
			execCommand("git", "add", filePath).Run()
		}
	case "Skip":
		fmt.Fprintln(cmd.OutOrStdout(), "Skipped.")
	case "Abort":
		return fmt.Errorf("aborted by user")
	}

	return nil
}
