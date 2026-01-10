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

func init() {
	rootCmd.AddCommand(inspectCmd)
}

var inspectCmd = &cobra.Command{
	Use:   "inspect <session-name>",
	Short: "Display a comprehensive summary of a session",
	Long:  `Provides a non-interactive, detailed summary of a specific RECAC session, including metadata, token usage, cost, and log excerpts.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
		}

		return displaySessionSummary(cmd, session)
	},
}

func displaySessionSummary(cmd *cobra.Command, session *runner.SessionState) error {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	// Metadata
	fmt.Fprintln(w, "METADATA")
	fmt.Fprintf(w, "  Name:\t%s\n", session.Name)
	fmt.Fprintf(w, "  Status:\t%s\n", session.Status)
	fmt.Fprintf(w, "  Start Time:\t%s\n", session.StartTime.Format(time.RFC3339))
	if !session.EndTime.IsZero() {
		fmt.Fprintf(w, "  End Time:\t%s\n", session.EndTime.Format(time.RFC3339))
		fmt.Fprintf(w, "  Duration:\t%s\n", session.EndTime.Sub(session.StartTime).Round(time.Second))
	} else {
		fmt.Fprintf(w, "  Duration:\t%s\n", time.Since(session.StartTime).Round(time.Second))
	}
	if session.Error != "" {
		fmt.Fprintf(w, "  Error:\t%s\n", session.Error)
	}
	fmt.Fprintln(w)

	// Agent and Token Stats
	if session.AgentStateFile != "" {
		agentState, err := loadAgentState(session.AgentStateFile)
		if err != nil {
			if !os.IsNotExist(err) {
				return fmt.Errorf("could not load agent state: %w", err)
			}
		} else {
			fmt.Fprintln(w, "AGENT & TOKEN STATS")
			fmt.Fprintf(w, "  Model:\t%s\n", agentState.Model)
			fmt.Fprintf(w, "  Prompt Tokens:\t%d\n", agentState.TokenUsage.TotalPromptTokens)
			fmt.Fprintf(w, "  Completion Tokens:\t%d\n", agentState.TokenUsage.TotalResponseTokens)
			fmt.Fprintf(w, "  Total Tokens:\t%d\n", agentState.TokenUsage.TotalTokens)
			cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)
			fmt.Fprintf(w, "  Estimated Cost:\t$%.6f\n", cost)
			fmt.Fprintln(w)
		}
	}

	// Log Excerpt
	logFile, err := os.ReadFile(session.LogFile)
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to read log file: %w", err)
		}
	} else {
		fmt.Fprintln(w, "LOG EXCERPT (LAST 10 LINES)")
		lines := tail(string(logFile), 10)
		for _, line := range lines {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}

	return w.Flush()
}

func tail(s string, n int) []string {
	// Trim trailing newline to avoid an extra empty line after split
	s = strings.TrimRight(s, "\n")
	lines := strings.Split(s, "\n")
	if len(lines) <= n {
		return lines
	}
	return lines[len(lines)-n:]
}
