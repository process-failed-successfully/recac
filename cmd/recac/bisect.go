package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	bisectGood     string
	bisectBad      string
	bisectCommand  string
	bisectAICheck  bool
	bisectMaxSteps int = 100 // Package-level variable for testing
)

var bisectCmd = &cobra.Command{
	Use:   "bisect",
	Short: "Automated git bisect with optional AI verification",
	Long: `Automates the git bisect process to find the commit that introduced a bug.
You can specify a command to run at each step, and optionally use an AI agent
to interpret the output of that command (useful for flaky tests or complex error messages).

Example:
  recac bisect --bad HEAD --good v1.0 --command "go test ./..." --ai-check
`,
	RunE: runBisectCmd,
}

func init() {
	bisectCmd.Flags().StringVar(&bisectGood, "good", "", "Known good commit (required)")
	bisectCmd.Flags().StringVar(&bisectBad, "bad", "HEAD", "Known bad commit")
	bisectCmd.Flags().StringVar(&bisectCommand, "command", "", "Command to run at each step (required)")
	bisectCmd.Flags().BoolVar(&bisectAICheck, "ai-check", false, "Use AI to verify command output")

	bisectCmd.MarkFlagRequired("good")
	bisectCmd.MarkFlagRequired("command")

	rootCmd.AddCommand(bisectCmd)
}

func runBisectCmd(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %w", err)
	}

	gitClient := gitClientFactory()
	if !gitClient.RepoExists(cwd) {
		return fmt.Errorf("current directory is not a git repository")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Starting bisect: Good=%s, Bad=%s\n", bisectGood, bisectBad)
	if err := gitClient.BisectStart(cwd, bisectBad, bisectGood); err != nil {
		return fmt.Errorf("failed to start bisect: %w", err)
	}

	// Ensure we reset on exit
	defer func() {
		fmt.Fprintln(cmd.OutOrStdout(), "Resetting bisect session...")
		if err := gitClient.BisectReset(cwd); err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "Warning: failed to reset bisect: %v\n", err)
		}
	}()

	visited := make(map[string]bool)

	for i := 0; i < bisectMaxSteps; i++ {
		// Get current commit to track progress
		currentSHA, err := gitClient.CurrentCommitSHA(cwd)
		if err != nil {
			return fmt.Errorf("failed to get current commit SHA: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Step %d: Checking commit %s\n", i+1, currentSHA)

		if visited[currentSHA] {
			return fmt.Errorf("infinite loop detected: already visited commit %s", currentSHA)
		}
		visited[currentSHA] = true

		// Run verification command
		output, cmdErr := executeBisectCommand(cwd, bisectCommand)

		isGood := false
		if bisectAICheck {
			isGood, err = checkWithAI(ctx, cwd, output, cmdErr)
			if err != nil {
				return fmt.Errorf("AI check failed: %w", err)
			}
		} else {
			// Standard mode: Exit code 0 is Good, non-zero is Bad
			isGood = (cmdErr == nil)
		}

		status := "BAD"
		if isGood {
			status = "GOOD"
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Result: %s\n", status)

		if isGood {
			err = gitClient.BisectGood(cwd, "")
		} else {
			err = gitClient.BisectBad(cwd, "")
		}

		if err != nil {
			return fmt.Errorf("git bisect command failed: %w", err)
		}

		// Check if we moved
		newSHA, shaErr := gitClient.CurrentCommitSHA(cwd)
		if shaErr != nil {
			return fmt.Errorf("failed to get current commit SHA: %w", shaErr)
		}
		if newSHA == currentSHA {
			// We didn't move. Likely found the bad commit or stuck.
			// Since we duplicate visited check, we can stop here.
			fmt.Fprintf(cmd.OutOrStdout(), "Bisect converged or stopped at %s.\n", newSHA)
			return nil
		}
	}

	return fmt.Errorf("bisect did not complete within %d steps", bisectMaxSteps)
}

func checkWithAI(ctx context.Context, cwd, output string, cmdErr error) (bool, error) {
	projectPath, _ := os.Getwd()
	projectName := filepath.Base(projectPath)

	agentClient, err := agentClientFactory(ctx, viper.GetString("provider"), viper.GetString("model"), projectPath, projectName)
	if err != nil {
		return false, fmt.Errorf("failed to initialize agent: %w", err)
	}

	prompt := fmt.Sprintf(`You are helping verify a git bisect step.
I ran the command: "%s"

Here is the output:
%s

Command Exit Code: %v

Is this output indicating a SUCCESS (GOOD) or FAILURE (BAD)?
Consider the command exit code, but prioritize the output content if the exit code is misleading (e.g. some tests fail but are ignored).
Respond with ONLY "GOOD" or "BAD".`, bisectCommand, output, cmdErr)

	response, err := agentClient.Send(ctx, prompt)
	if err != nil {
		return false, err
	}

	normalized := strings.ToUpper(strings.TrimSpace(response))
	if strings.Contains(normalized, "GOOD") {
		return true, nil
	}
	if strings.Contains(normalized, "BAD") {
		return false, nil
	}

	return false, fmt.Errorf("AI returned ambiguous response: %s", response)
}

// executeBisectCommand executes a command string in the shell
func executeBisectCommand(dir, command string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd", "/C", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}
