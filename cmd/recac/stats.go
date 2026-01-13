package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

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

		// Get filter flags
		status, _ := cmd.Flags().GetString("status")
		since, _ := cmd.Flags().GetString("since")
		model, _ := cmd.Flags().GetString("model")

		stats, err := calculateStats(sm, status, since, model)
		if err != nil {
			return fmt.Errorf("could not calculate statistics: %w", err)
		}

		displayStats(stats)
		return nil
	},
}

func calculateStats(sm ISessionManager, statusFilter string, sinceFilter string, modelFilter string) (*AggregateStats, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("could not list sessions: %w", err)
	}

	var sinceCutoff time.Time
	if sinceFilter != "" {
		duration, err := time.ParseDuration(sinceFilter)
		if err != nil {
			return nil, fmt.Errorf("invalid duration format for --since: %w", err)
		}
		sinceCutoff = time.Now().Add(-duration)
	}

	stats := &AggregateStats{
		StatusCounts: make(map[string]int),
	}

	for _, session := range sessions {
		// Apply filters
		if statusFilter != "" && session.Status != statusFilter {
			continue
		}
		if !sinceCutoff.IsZero() && session.StartTime.Before(sinceCutoff) {
			continue
		}

		agentState, err := loadAgentState(session.AgentStateFile)
		if modelFilter != "" {
			if session.AgentStateFile == "" {
				continue
			}
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, fmt.Errorf("could not load agent state for session %s: %w", session.Name, err)
			}
			if agentState.Model != modelFilter {
				continue
			}
		}

		stats.TotalSessions++
		stats.StatusCounts[session.Status]++

		if session.AgentStateFile == "" {
			continue
		}

		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("could not load agent state for session %s: %w", session.Name, err)
		}

		stats.TotalTokens += agentState.TokenUsage.TotalTokens
		stats.TotalPromptTokens += agentState.TokenUsage.TotalPromptTokens
		stats.TotalResponseTokens += agentState.TokenUsage.TotalResponseTokens
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
	statsCmd.Flags().String("status", "", "Filter by session status (e.g., COMPLETED, FAILED)")
	statsCmd.Flags().String("since", "", "Filter sessions started within a duration (e.g., 24h, 7d)")
	statsCmd.Flags().String("model", "", "Filter by agent model name")
}
