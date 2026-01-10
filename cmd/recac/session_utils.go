package main

import (
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/runner"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

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

