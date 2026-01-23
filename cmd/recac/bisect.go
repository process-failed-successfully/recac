package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"recac/internal/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	bisectGood   string
	bisectBad    string
	bisectScript string
)

var bisectCmd = &cobra.Command{
	Use:   "bisect [bug description]",
	Short: "Automated git bisect using AI",
	Long: `Automates the git bisect process to find the commit that introduced a bug.
It uses AI to generate a reproduction script based on your bug description,
validates the script, and then runs git bisect.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBisect,
}

func init() {
	rootCmd.AddCommand(bisectCmd)
	bisectCmd.Flags().StringVar(&bisectGood, "good", "", "Commit hash known to be good (required)")
	bisectCmd.Flags().StringVar(&bisectBad, "bad", "HEAD", "Commit hash known to be bad")
	bisectCmd.Flags().StringVar(&bisectScript, "script", "", "Path to a manual reproduction script (skips AI generation)")
}

func runBisect(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	gitClient := gitClientFactory()
	if !gitClient.RepoExists(cwd) {
		return fmt.Errorf("not a git repository")
	}

	// 1. Determine Good/Bad commits
	badCommit, err := gitClient.CurrentCommitSHA(cwd)
	if err != nil {
		return fmt.Errorf("failed to get current commit: %w", err)
	}
	if bisectBad != "HEAD" {
		badCommit = bisectBad
	}

	goodCommit := bisectGood
	if goodCommit == "" {
		// Try to guess or ask?
		// For now, fail if not provided
		return fmt.Errorf("please provide a good commit using --good")
	}

	// 2. Get Reproduction Script
	scriptPath := bisectScript
	if scriptPath == "" {
		if len(args) == 0 {
			return fmt.Errorf("please provide a bug description or a --script")
		}
		bugDesc := args[0]

		fmt.Fprintf(cmd.OutOrStdout(), "🤖 Generating reproduction script for: %s\n", bugDesc)

		generatedScript, err := generateReproScript(ctx, cwd, bugDesc)
		if err != nil {
			return err
		}

		scriptPath = filepath.Join(cwd, "repro.sh")
		if err := os.WriteFile(scriptPath, []byte(generatedScript), 0755); err != nil {
			return fmt.Errorf("failed to write repro script: %w", err)
		}
		defer func() {
			// Cleanup script? Maybe keep it if failed or asked
			// os.Remove(scriptPath)
		}()
		fmt.Fprintf(cmd.OutOrStdout(), "Generated %s\n", scriptPath)
	} else {
		// Ensure absolute path
		if !filepath.IsAbs(scriptPath) {
			scriptPath = filepath.Join(cwd, scriptPath)
		}
		// Ensure executable
		if err := os.Chmod(scriptPath, 0755); err != nil {
			return fmt.Errorf("failed to make script executable: %w", err)
		}
	}

	// 3. Validate Script
	fmt.Fprintln(cmd.OutOrStdout(), "🕵️ Validating script...")

	// Check Bad (Should Fail)
	// We might need to checkout bad commit first if we are not on it
	current, _ := gitClient.CurrentCommitSHA(cwd)
	if current != badCommit {
		if err := gitClient.Checkout(cwd, badCommit); err != nil {
			return fmt.Errorf("failed to checkout bad commit: %w", err)
		}
	}

	out, err := execCommand(scriptPath).CombinedOutput()
	if err == nil {
		return fmt.Errorf("script passed on BAD commit %s (expected failure)\nOutput:\n%s", badCommit, string(out))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "✅ Script failed on BAD commit (as expected).")

	// Check Good (Should Pass)
	if err := gitClient.Checkout(cwd, goodCommit); err != nil {
		return fmt.Errorf("failed to checkout good commit: %w", err)
	}

	out, err = execCommand(scriptPath).CombinedOutput()
	if err != nil {
		// Restore
		gitClient.Checkout(cwd, badCommit)
		return fmt.Errorf("script failed on GOOD commit %s (expected pass)\nOutput:\n%s", goodCommit, string(out))
	}
	fmt.Fprintln(cmd.OutOrStdout(), "✅ Script passed on GOOD commit (as expected).")

	// Restore to bad (start point)
	gitClient.Checkout(cwd, badCommit)

	// 4. Run Bisect
	fmt.Fprintln(cmd.OutOrStdout(), "🚀 Starting git bisect...")
	if err := gitClient.BisectStart(cwd, badCommit, goodCommit); err != nil {
		gitClient.BisectReset(cwd)
		return fmt.Errorf("bisect start failed: %w", err)
	}

	// Bisect Run
	output, err := gitClient.BisectRun(cwd, scriptPath)
	if err != nil {
		gitClient.BisectReset(cwd)
		return fmt.Errorf("bisect run failed: %w", err)
	}

	// Parse Output to find the bad commit
	// Output usually ends with "sha1 is the first bad commit"
	lines := strings.Split(strings.TrimSpace(output), "\n")
	var culprit string
	for _, line := range lines {
		if strings.Contains(line, "is the first bad commit") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				culprit = parts[0]
			}
		}
	}

	gitClient.BisectReset(cwd)

	if culprit == "" {
		fmt.Fprintln(cmd.OutOrStdout(), "❌ Could not identify the bad commit.")
		fmt.Fprintln(cmd.OutOrStdout(), output)
		return nil
	}

	fmt.Fprintf(cmd.OutOrStdout(), "\n🎯 Culprit found: %s\n", culprit)

	// 5. Explain
	fmt.Fprintln(cmd.OutOrStdout(), "🧠 Analyzing culprit...")
	explanation, err := explainCommit(ctx, cwd, culprit)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to explain commit: %v\n", err)
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\nExplanation:")
		fmt.Fprintln(cmd.OutOrStdout(), explanation)
	}

	return nil
}

func generateReproScript(ctx context.Context, cwd, bugDesc string) (string, error) {
	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-bisect-gen")
	if err != nil {
		return "", fmt.Errorf("failed to create agent: %w", err)
	}

	prompt := fmt.Sprintf(`Create a bash script (repro.sh) that reproduces the following bug.
The script should exit with code 0 if the bug is ABSENT (good), and exit with code 1 (or non-zero) if the bug is PRESENT (bad).
Assume the script runs in the root of the repository.
Do not assume any external dependencies other than standard linux tools, go, curl, etc.

Bug Description:
%s

Output ONLY the bash script content.`, bugDesc)

	resp, err := ag.Send(ctx, prompt)
	if err != nil {
		return "", err
	}

	return utils.CleanCodeBlock(resp), nil
}

func explainCommit(ctx context.Context, cwd, commitSHA string) (string, error) {
	gitClient := gitClientFactory()
	diff, err := gitClient.Diff(cwd, commitSHA+"~1", commitSHA)
	if err != nil {
		return "", err
	}

	// Limit diff size
	if len(diff) > 10000 {
		diff = diff[:10000] + "...(truncated)"
	}

	log, _ := gitClient.Log(cwd, "-n", "1", commitSHA)
	commitMsg := ""
	if len(log) > 0 {
		commitMsg = log[0]
	}

	provider := viper.GetString("provider")
	model := viper.GetString("model")

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-bisect-explain")
	if err != nil {
		return "", err
	}

	prompt := fmt.Sprintf(`Analyze this commit which introduced a bug.
Explain why it might have caused the issue.

Commit: %s
Message: %s

Diff:
%s`, commitSHA, commitMsg, diff)

	return ag.Send(ctx, prompt)
}
