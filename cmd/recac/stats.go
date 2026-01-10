package main

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"recac/internal/agent"
)

// AggregateStats holds the calculated statistics
type AggregateStats struct {
	TotalSessions       int
	TotalTokens         int
	TotalPromptTokens   int
	TotalResponseTokens int
	TotalCost           float64
	StatusCounts        map[string]int
}

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show aggregate statistics for all sessions",
	Long:  `Calculates and displays aggregate statistics from all session history files, such as total tokens used, total cost, and a breakdown of session statuses.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("could not create session manager: %w", err)
		}

		stats, err := calculateStats(sm)
		if err != nil {
			return fmt.Errorf("could not calculate statistics: %w", err)
		}

		displayStats(stats)
		return nil
	},
}

func calculateStats(sm ISessionManager) (*AggregateStats, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("could not list sessions: %w", err)
	}

	stats := &AggregateStats{
		StatusCounts: make(map[string]int),
	}

	for _, session := range sessions {
		stats.TotalSessions++
		stats.StatusCounts[session.Status]++

		if session.AgentStateFile == "" {
			continue
		}

		agentState, err := LoadAgentState(session.AgentStateFile)
		if err != nil {
			// If the agent state file doesn't exist, we can just skip it
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("could not load agent state for session %s: %w", session.Name, err)
		}

		stats.TotalTokens += agentState.TokenUsage.TotalTokens
		stats.TotalPromptTokens += agentState.TokenUsage.TotalPromptTokens
		stats.TotalResponseTokens += agentState.TokenUsage.TotalResponseTokens

		// Calculate cost
		stats.TotalCost += agent.CalculateCost(agentState.Model, agentState.TokenUsage)
	}

	return stats, nil
}

func displayStats(stats *AggregateStats) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "AGGREGATE SESSION STATISTICS")
	fmt.Fprintln(w, "----------------------------")
	fmt.Fprintf(w, "Total Sessions:\t%d\n", stats.TotalSessions)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Token Usage:")
	fmt.Fprintf(w, "  Total Tokens:\t%d\n", stats.TotalTokens)
	fmt.Fprintf(w, "  Prompt Tokens:\t%d\n", stats.TotalPromptTokens)
	fmt.Fprintf(w, "  Response Tokens:\t%d\n", stats.TotalResponseTokens)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Cost:")
	fmt.Fprintf(w, "  Total Estimated Cost:\t$%.4f\n", stats.TotalCost)
	fmt.Fprintln(w, "")
	fmt.Fprintln(w, "Session Status Breakdown:")
	for status, count := range stats.StatusCounts {
		fmt.Fprintf(w, "  %s:\t%d\n", status, count)
	}
	w.Flush()
}

func init() {
	rootCmd.AddCommand(statsCmd)
}
