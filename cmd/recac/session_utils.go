package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"recac/internal/agent"
	"recac/internal/git"
	"recac/internal/runner"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// gitClient defines the interface for git operations, allowing for mocking in tests.
type gitClient interface {
	Diff(workspace, fromSHA, toSHA string) (string, error)
	CurrentCommitSHA(workspace string) (string, error)
}

// gitNewClient is a factory function that can be overridden in tests.
var gitNewClient = func() gitClient {
	// We need to wrap the concrete client in the interface type.
	return git.NewClient()
}

// DisplaySessionDetail prints a detailed view of a single session.
func DisplaySessionDetail(cmd *cobra.Command, session *runner.SessionState, fullLogs bool) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Session Details for '%s'\n", session.Name)
	fmt.Fprintln(w, "-------------------------")
	fmt.Fprintf(w, "Name:\t%s\n", session.Name)
	fmt.Fprintf(w, "Status:\t%s\n", session.Status)
	fmt.Fprintf(w, "PID:\t%d\n", session.PID)
	fmt.Fprintf(w, "Type:\t%s\n", session.Type)
	fmt.Fprintf(w, "Start Time:\t%s\n", session.StartTime.Format(time.RFC1123))
	if !session.EndTime.IsZero() {
		fmt.Fprintf(w, "End Time:\t%s\n", session.EndTime.Format(time.RFC1123))
		fmt.Fprintf(w, "Duration:\t%s\n", session.EndTime.Sub(session.StartTime).Round(time.Second))
	}
	fmt.Fprintf(w, "Workspace:\t%s\n", session.Workspace)
	fmt.Fprintf(w, "Log File:\t%s\n", session.LogFile)
	if session.Error != "" {
		fmt.Fprintf(w, "Error:\t%s\n", session.Error)
	}
	w.Flush()

	if session.AgentStateFile != "" {
		agentState, err := loadAgentState(session.AgentStateFile)
		if err == nil && agentState != nil {
			fmt.Fprintln(w, "\nAgent & Token Usage")
			fmt.Fprintln(w, "-------------------")
			fmt.Fprintf(w, "Model:\t%s\n", agentState.Model)
			fmt.Fprintf(w, "Prompt Tokens:\t%d\n", agentState.TokenUsage.TotalPromptTokens)
			fmt.Fprintf(w, "Completion Tokens:\t%d\n", agentState.TokenUsage.TotalResponseTokens)
			fmt.Fprintf(w, "Total Tokens:\t%d\n", agentState.TokenUsage.TotalTokens)
			cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)
			fmt.Fprintf(w, "Estimated Cost:\t$%.6f\n", cost)
			w.Flush()
		} else if !os.IsNotExist(err) {
			// Only show error if the file exists but is invalid.
			fmt.Fprintf(cmd.ErrOrStderr(), "\nWarning: Could not load agent state from %s: %v\n", session.AgentStateFile, err)
		}
	}

	// Git Diff Stat
	sm, err := sessionManagerFactory()
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Failed to create session manager to get git diff: %v\n", err)
	} else {
		diffStat, err := sm.GetSessionGitDiffStat(session.Name)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "\nWarning: Could not generate git diff stat: %v\n", err)
		} else if diffStat != "" {
			fmt.Fprintln(w, "\nGit Changes (stat)")
			fmt.Fprintln(w, "------------------")
			w.Flush() // Flush before writing raw content
			cmd.Println(diffStat)
		}
	}

	if _, err := os.Stat(session.LogFile); err == nil {
		logContent, err := os.ReadFile(session.LogFile)
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(logContent)), "\n")
			if fullLogs {
				fmt.Fprintln(w, "\nFull Logs")
				fmt.Fprintln(w, "-----------")
				w.Flush()
				cmd.Println(string(logContent))
			} else {
				fmt.Fprintln(w, "\nRecent Logs (last 10 lines)")
				fmt.Fprintln(w, "---------------------------")
				w.Flush()
				start := 0
				if len(lines) > 10 {
					start = len(lines) - 10
				}
				for _, line := range lines[start:] {
					cmd.Println(line)
				}
			}
		} else {
			cmd.PrintErrf("Failed to read log file: %v\n", err)
		}
	}
	return nil
}

func DisplaySessionDiff(cmd *cobra.Command, sessionA, sessionB *runner.SessionState) error {
	cmd.Println("📊 Metadata Comparison")
	printMetadataDiff(cmd, sessionA, sessionB)

	cmd.Println("\n📜 Log Diff")
	return printLogDiff(cmd, sessionA.LogFile, sessionB.LogFile)
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

// loadAgentState is a helper to read and parse an agent state file.
var loadAgentState = func(filePath string) (*agent.State, error) {
	if filePath == "" {
		return nil, fmt.Errorf("file path is empty")
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	var state agent.State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, err
	}
	return &state, nil
}
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
