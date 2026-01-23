package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	bisectGood      string
	bisectBad       string
	bisectCommand   string
	bisectAICheck   string
	bisectAutoReset bool
	bisectMaxSteps  = 100
)

var bisectCmd = &cobra.Command{
	Use:   "bisect",
	Short: "Automated git bisect with AI verification and explanation",
	Long: `Automates finding the commit that introduced a bug (regression).
It wraps 'git bisect' but allows you to use an AI agent to determine if a commit is 'good' or 'bad' based on command output.
Once the bad commit is found, the AI explains the regression.

Examples:
  # Simple bisect with a test command
  recac bisect --good v1.0 --bad HEAD --command "go test ./..."

  # Bisect with AI verification (e.g. check logs for specific failure)
  recac bisect --good a1b2c3d --command "curl -v localhost:8080" --ai-check "Did the server respond with 500?"
`,
	RunE: runBisect,
}

func init() {
	rootCmd.AddCommand(bisectCmd)
	bisectCmd.Flags().StringVar(&bisectGood, "good", "", "Commit hash known to be good (required)")
	bisectCmd.Flags().StringVar(&bisectBad, "bad", "HEAD", "Commit hash known to be bad")
	bisectCmd.Flags().StringVar(&bisectCommand, "command", "", "Command to run at each step (required)")
	bisectCmd.Flags().StringVar(&bisectAICheck, "ai-check", "", "Prompt for AI to verify output (optional)")
	bisectCmd.Flags().BoolVar(&bisectAutoReset, "auto-reset", false, "Automatically reset git bisect state after finishing")
}

func runBisect(cmd *cobra.Command, args []string) error {
	if bisectGood == "" {
		return errors.New("--good commit is required")
	}
	if bisectCommand == "" {
		return errors.New("--command is required")
	}

	// 1. Check for uncommitted changes
	if err := checkCleanWorkingTree(); err != nil {
		return fmt.Errorf("working tree is not clean: %w. Please commit or stash changes before bisecting", err)
	}

	fmt.Fprintln(cmd.OutOrStdout(), "🏁 Starting AI Bisect...")

	// 2. Start Bisect
	output, err := runBisectGitHelper("bisect", "start")
	if err != nil {
		return fmt.Errorf("failed to start bisect: %s", output)
	}

	// Always try to reset if we crash (if user wants, but actually for safety we might just leave it)
	// But if we finish successfully and --auto-reset is on, we reset then.

	// 3. Set Bad and Good
	fmt.Fprintf(cmd.OutOrStdout(), "📍 Setting bad commit: %s\n", bisectBad)
	output, err = runBisectGitHelper("bisect", "bad", bisectBad)
	if err != nil {
		_, _ = runBisectGitHelper("bisect", "reset")
		return fmt.Errorf("failed to set bad commit: %s", output)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "📍 Setting good commit: %s\n", bisectGood)
	output, err = runBisectGitHelper("bisect", "good", bisectGood)
	if err != nil {
		_, _ = runBisectGitHelper("bisect", "reset")
		return fmt.Errorf("failed to set good commit: %s", output)
	}

	// output might contain "is the first bad commit" if it was immediate? Unlikely for bad != good.
	// output usually: "Bisecting: X revisions left..."

	// 4. Bisect Loop
	foundCommit := ""
	maxSteps := bisectMaxSteps
	step := 0

	// Check if we are already done (rare)
	if strings.Contains(output, "is the first bad commit") {
		foundCommit = parseBadCommit(output)
	} else {
		for step < maxSteps {
			step++
			fmt.Fprintf(cmd.OutOrStdout(), "\n🔄 Bisect Step %d...\n", step)

			// Run verification
			result, _, err := verifyCommit(cmd, bisectCommand, bisectAICheck)
			if err != nil {
				// Execution error (e.g. command not found, or AI failed).
				// We might want to abort or skip.
				fmt.Fprintf(cmd.ErrOrStderr(), "⚠️ Verification failed execution: %v. Skipping commit.\n", err)
				result = "skip"
			}

			fmt.Fprintf(cmd.OutOrStdout(), "👉 Verdict: %s\n", strings.ToUpper(result))

			// Tell Git
			out, err := runBisectGitHelper("bisect", result)
			if err != nil {
				return fmt.Errorf("git bisect failed: %s", out)
			}
			fmt.Fprintln(cmd.OutOrStdout(), strings.TrimSpace(out))

			if strings.Contains(out, "is the first bad commit") {
				foundCommit = parseBadCommit(out)
				break
			}

			if strings.Contains(out, "No testable commit found") {
				return errors.New("bisect failed: no testable commit found")
			}
		}
	}

	if foundCommit == "" {
		return errors.New("bisect failed or exceeded max steps without finding culprit")
	}

	// 5. Found! Explain it.
	fmt.Fprintln(cmd.OutOrStdout(), "\n🕵️ Culprit Found!")
	fmt.Fprintf(cmd.OutOrStdout(), "commit %s\n", foundCommit)

	if err := explainCulprit(cmd, foundCommit); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to explain commit: %v\n", err)
	}

	// 6. Reset if requested
	if bisectAutoReset {
		runBisectGitHelper("bisect", "reset")
		fmt.Fprintln(cmd.OutOrStdout(), "Start reset.")
	} else {
		fmt.Fprintln(cmd.OutOrStdout(), "\nNote: You are still in bisect state at the bad commit.")
		fmt.Fprintln(cmd.OutOrStdout(), "Run 'git bisect reset' to return to original branch.")
	}

	return nil
}

func checkCleanWorkingTree() error {
	// git diff-index --quiet HEAD
	cmd := execCommand("git", "diff-index", "--quiet", "HEAD")
	return cmd.Run()
}

func runBisectGitHelper(args ...string) (string, error) {
	cmd := execCommand("git", args...)
	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr
	err := cmd.Run()
	output := out.String() + stderr.String()
	if err != nil {
		return output, err
	}
	return output, nil
}

func verifyCommit(cmd *cobra.Command, command string, aiPrompt string) (string, string, error) {
	// Run the command
	// We use shell to allow complex commands
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = execCommand("cmd", "/C", command)
	} else {
		c = execCommand("sh", "-c", command)
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	c.Stdout = &out
	c.Stderr = &stderr
	err := c.Run()
	output := out.String() + "\n" + stderr.String()

	// If AI check is enabled
	if aiPrompt != "" {
		// Ask AI
		isGood, err := askAIVerdict(cmd, aiPrompt, output, err)
		if err != nil {
			return "skip", output, err
		}
		if isGood {
			return "good", output, nil
		}
		return "bad", output, nil
	}

	// Standard exit code check
	if err == nil {
		return "good", output, nil
	}
	// exit code != 0
	return "bad", output, nil
}

func askAIVerdict(cmd *cobra.Command, prompt, output string, cmdErr error) (bool, error) {
	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-bisect")
	if err != nil {
		return false, err
	}

	exitCode := 0
	if cmdErr != nil {
		exitCode = 1 // Simplified
		if exitErr, ok := cmdErr.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
	}

	fullPrompt := fmt.Sprintf(`I am running a git bisect to find a regression.
I ran the command.
Exit Code: %d
Output:
'''
%s
'''

My criteria for "Success" (Good State) is: "%s"

Based on the output and criteria, is this commit GOOD or BAD?
Reply with just "GOOD" or "BAD". If you cannot tell (e.g. build error unrelated to logic), reply "SKIP".`, exitCode, output, prompt)

	resp, err := ag.Send(ctx, fullPrompt)
	if err != nil {
		return false, err
	}

	resp = strings.TrimSpace(strings.ToUpper(resp))
	if strings.Contains(resp, "GOOD") {
		return true, nil
	}
	if strings.Contains(resp, "BAD") {
		return false, nil
	}
	if strings.Contains(resp, "SKIP") {
		return false, errors.New("AI decided to skip")
	}

	// Fallback
	return false, fmt.Errorf("unknown AI response: %s", resp)
}

func parseBadCommit(output string) string {
	// Format: "d839... is the first bad commit"
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "is the first bad commit") {
			parts := strings.Fields(line)
			if len(parts) > 0 {
				return parts[0]
			}
		}
	}
	return ""
}

func explainCulprit(cmd *cobra.Command, commitSHA string) error {
	// Get commit info
	info, err := runBisectGitHelper("show", "--stat", commitSHA)
	if err != nil {
		return err
	}

	// Get diff (limited size)
	diff, err := runBisectGitHelper("show", commitSHA)
	if err != nil {
		return err
	}
	if len(diff) > 5000 {
		diff = diff[:5000] + "\n... (truncated)"
	}

	ctx := context.Background()
	provider := viper.GetString("provider")
	model := viper.GetString("model")
	cwd, _ := os.Getwd()

	ag, err := agentClientFactory(ctx, provider, model, cwd, "recac-bisect-explain")
	if err != nil {
		return err
	}

	prompt := fmt.Sprintf(`The following commit has been identified as the cause of a regression.
Please explain what this commit changed and why it might have caused the issue.

Commit Info:
%s

Diff:
%s`, info, diff)

	fmt.Fprintln(cmd.OutOrStdout(), "\n🧠 AI Analysis of the Culprit:")
	_, err = ag.SendStream(ctx, prompt, func(chunk string) {
		fmt.Fprint(cmd.OutOrStdout(), chunk)
	})
	fmt.Println()

	return err
}
