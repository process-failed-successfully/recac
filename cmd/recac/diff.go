package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)



var diffCmd = &cobra.Command{
	Use:   "diff [session_a] [session_b]",
	Short: "Compare two sessions",
	Long:  "Compares two sessions.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionAName := args[0]
		sessionBName := args[1]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		sessionA, err := sm.LoadSession(sessionAName)
		if err != nil {
			return fmt.Errorf("failed to load session %s: %w", sessionAName, err)
		}

		sessionB, err := sm.LoadSession(sessionBName)
		if err != nil {
			return fmt.Errorf("failed to load session %s: %w", sessionBName, err)
		}

		// Print Metadata Comparison
		cmd.Println("📊 Metadata Comparison")
		printMetadataDiff(cmd, sessionA, sessionB)

		// Print Log Diff
		cmd.Println("\n📜 Log Diff")
		return printLogDiff(cmd, sessionA.LogFile, sessionB.LogFile)
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}

func printMetadataDiff(cmd *cobra.Command, sA, sB *runner.SessionState) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "METRIC\tSESSION A\tSESSION B")
	fmt.Fprintln(w, "------\t---------\t---------")
	fmt.Fprintf(w, "Name\t%s\t%s\n", sA.Name, sB.Name)
	fmt.Fprintf(w, "Status\t%s\t%s\n", sA.Status, sB.Status)

	// Durations
	durationA := sA.EndTime.Sub(sA.StartTime).Round(time.Second)
	if sA.Status == "running" {
		durationA = time.Since(sA.StartTime).Round(time.Second)
	}
	durationB := sB.EndTime.Sub(sB.StartTime).Round(time.Second)
	if sB.Status == "running" {
		durationB = time.Since(sB.StartTime).Round(time.Second)
	}
	fmt.Fprintf(w, "Duration\t%s\t%s\n", durationA, durationB)

	// Agent State & Cost
	stateA, errA := loadAgentState(sA.AgentStateFile)
	stateB, errB := loadAgentState(sB.AgentStateFile)

	costA, costB := "$0.00", "$0.00"
	tokensA, tokensB := "0", "0"
	modelA, modelB := "N/A", "N/A"

	if errA == nil && stateA != nil {
		cost := agent.CalculateCost(stateA.Model, stateA.TokenUsage)
		costA = fmt.Sprintf("$%.4f", cost)
		tokensA = fmt.Sprintf("%d", stateA.TokenUsage.TotalPromptTokens+stateA.TokenUsage.TotalResponseTokens)
		modelA = stateA.Model
	}
	if errB == nil && stateB != nil {
		cost := agent.CalculateCost(stateB.Model, stateB.TokenUsage)
		costB = fmt.Sprintf("$%.4f", cost)
		tokensB = fmt.Sprintf("%d", stateB.TokenUsage.TotalPromptTokens+stateB.TokenUsage.TotalResponseTokens)
		modelB = stateB.Model
	}
	fmt.Fprintf(w, "Model\t%s\t%s\n", modelA, modelB)
	fmt.Fprintf(w, "Tokens\t%s\t%s\n", tokensA, tokensB)
	fmt.Fprintf(w, "Cost\t%s\t%s\n", costA, costB)

	// Error
	errStrA := "None"
	if sA.Error != "" {
		errStrA = sA.Error
	}
	errStrB := "None"
	if sB.Error != "" {
		errStrB = sB.Error
	}
	fmt.Fprintf(w, "Error\t%s\t%s\n", errStrA, errStrB)

	w.Flush()
}

func printLogDiff(cmd *cobra.Command, logA, logB string) error {
	diffCmd := exec.Command("diff", "-u", logA, logB)
	output, err := diffCmd.CombinedOutput()

	// diff exits with 1 if files differ, which is not an error for us.
	if err != nil {
		// If `diff` command is not found, use fallback
		if _, ok := err.(*exec.Error); ok {
			cmd.Println("(using fallback diff)")
			return fallbackDiff(cmd, logA, logB)
		}
		// Any other error from `diff` (besides exit code 1) is a problem
		if exitErr, ok := err.(*exec.ExitError); !ok || exitErr.ExitCode() != 1 {
			return fmt.Errorf("failed to execute diff command: %w\nOutput:\n%s", err, string(output))
		}
	}

	if len(output) == 0 {
		cmd.Println("No differences in logs.")
		return nil
	}

	// Colorize the output for better readability
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "+"):
			// Green for additions
			cmd.Printf("\x1b[32m%s\x1b[0m\n", line)
		case strings.HasPrefix(line, "-"):
			// Red for deletions
			cmd.Printf("\x1b[31m%s\x1b[0m\n", line)
		case strings.HasPrefix(line, "@@"):
			// Cyan for context lines
			cmd.Printf("\x1b[36m%s\x1b[0m\n", line)
		default:
			cmd.Println(line)
		}
	}

	return nil
}

// fallbackDiff provides a basic diff if the `diff` command is not available.
func fallbackDiff(cmd *cobra.Command, file1, file2 string) error {
	content1, err1 := os.ReadFile(file1)
	if err1 != nil {
		return fmt.Errorf("could not read file %s: %w", file1, err1)
	}
	content2, err2 := os.ReadFile(file2)
	if err2 != nil {
		return fmt.Errorf("could not read file %s: %w", file2, err2)
	}

	lines1 := strings.Split(string(content1), "\n")
	lines2 := strings.Split(string(content2), "\n")

	if string(content1) == string(content2) {
		cmd.Println("No differences in logs.")
		return nil
	}

	cmd.Println("--- ", file1)
	cmd.Println("+++ ", file2)

	// This is a very basic line-by-line comparison, not a true diff algorithm.
	for i := 0; i < len(lines1) || i < len(lines2); i++ {
		if i < len(lines1) && i < len(lines2) {
			if lines1[i] != lines2[i] {
				cmd.Printf("\x1b[31m- %s\x1b[0m\n", lines1[i])
				cmd.Printf("\x1b[32m+ %s\x1b[0m\n", lines2[i])
			} else {
				cmd.Println("  ", lines1[i])
			}
		} else if i < len(lines1) {
			cmd.Printf("\x1b[31m- %s\x1b[0m\n", lines1[i])
		} else if i < len(lines2) {
			cmd.Printf("\x1b[32m+ %s\x1b[0m\n", lines2[i])
		}
	}

	return nil
}
