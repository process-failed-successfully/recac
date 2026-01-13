package main

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/spf13/cobra"
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
	Short: "Show aggregate statistics for sessions with optional filters",
	Long: `Calculates and displays aggregate statistics from session history files, such as total tokens used, total cost, and a breakdown of session statuses.
Supports filtering by name, status, and time range to provide more targeted insights.`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("could not create session manager: %w", err)
		}

		allSessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("could not list sessions: %w", err)
		}

		// --- Apply Filters ---
		statusFilter, _ := cmd.Flags().GetString("status")
		nameFilter, _ := cmd.Flags().GetString("name")
		sinceFilter, _ := cmd.Flags().GetString("since")
		untilFilter, _ := cmd.Flags().GetString("until")

		var filteredSessions []*runner.SessionState
		for _, s := range allSessions {
			// Status filter
			if statusFilter != "" && !strings.EqualFold(s.Status, statusFilter) {
				continue
			}
			// Name filter
			if nameFilter != "" && !strings.Contains(s.Name, nameFilter) {
				continue
			}
			// Since filter
			if sinceFilter != "" {
				sinceTime, err := time.Parse(time.RFC3339, sinceFilter)
				if err != nil {
					return fmt.Errorf("invalid --since time format, please use RFC3339 (e.g., '2023-01-02T15:04:05Z'): %w", err)
				}
				if s.StartTime.Before(sinceTime) {
					continue
				}
			}
			// Until filter
			if untilFilter != "" {
				untilTime, err := time.Parse(time.RFC3339, untilFilter)
				if err != nil {
					return fmt.Errorf("invalid --until time format, please use RFC3339 (e.g., '2023-01-02T15:04:05Z'): %w", err)
				}
				if s.StartTime.After(untilTime) {
					continue
				}
			}
			filteredSessions = append(filteredSessions, s)
		}

		stats, err := calculateStats(filteredSessions)
		if err != nil {
			return fmt.Errorf("could not calculate statistics: %w", err)
		}

		displayStats(cmd, stats)
		return nil
	},
}

func calculateStats(sessions []*runner.SessionState) (*AggregateStats, error) {
	stats := &AggregateStats{
		StatusCounts: make(map[string]int),
	}

	for _, session := range sessions {
		stats.TotalSessions++
		stats.StatusCounts[session.Status]++

		if session.AgentStateFile == "" {
			continue
		}

		agentState, err := loadAgentState(session.AgentStateFile)
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

func displayStats(cmd *cobra.Command, stats *AggregateStats) {
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "AGGREGATE SESSION STATISTICS")
	fmt.Fprintln(w, "----------------------------")
	fmt.Fprintf(w, "Matching Sessions:\t%d\n", stats.TotalSessions)
	fmt.Fprintln(w, "")

	if stats.TotalSessions == 0 {
		cmd.Println("No sessions matched the specified filters.")
		return
	}

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
}

func init() {
	rootCmd.AddCommand(statsCmd)
	statsCmd.Flags().String("status", "", "Filter by session status (e.g., 'completed', 'error')")
	statsCmd.Flags().String("name", "", "Filter by session name (substring match)")
	statsCmd.Flags().String("since", "", "Filter sessions started after a specific time (RFC3339 format, e.g., '2023-01-02T15:04:05Z')")
	statsCmd.Flags().String("until", "", "Filter sessions started before a specific time (RFC3339 format, e.g., '2023-01-02T15:04:05Z')")
}
