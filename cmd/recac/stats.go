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

var (
	statusFilter string
	sinceFilter  string
	modelFilter  string
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show aggregate statistics for all sessions",
	Long: `Calculates and displays aggregate statistics from all session history files, such as total tokens used, total cost, and a breakdown of session statuses.
You can filter the sessions by status, time, or model.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("could not create session manager: %w", err)
		}

		var since time.Time
		if sinceFilter != "" {
			duration, err := time.ParseDuration(sinceFilter)
			if err != nil {
				return fmt.Errorf("invalid duration format for --since: %w", err)
			}
			since = time.Now().Add(-duration)
		}

		stats, err := calculateStats(sm, statusFilter, since, modelFilter)
		if err != nil {
			return fmt.Errorf("could not calculate statistics: %w", err)
		}

		displayStats(stats)
		return nil
	},
}

func calculateStats(sm ISessionManager, statusFilter string, sinceFilter time.Time, modelFilter string) (*AggregateStats, error) {
	sessions, err := sm.ListSessions()
	if err != nil {
		return nil, fmt.Errorf("could not list sessions: %w", err)
	}

	stats := &AggregateStats{
		StatusCounts: make(map[string]int),
	}

	for _, session := range sessions {
		// Apply filters that don't require loading the agent state file first
		if statusFilter != "" && session.Status != statusFilter {
			continue
		}
		if !sinceFilter.IsZero() && session.StartTime.Before(sinceFilter) {
			continue
		}

		// If filtering by model, we need to load the agent state.
		var agentState *agent.State
		if modelFilter != "" || session.AgentStateFile != "" {
			// Avoid loading if the file is empty
			if session.AgentStateFile == "" {
				if modelFilter != "" {
					continue
				}
			} else {
				var err error
				agentState, err = loadAgentState(session.AgentStateFile)
				if err != nil {
					if os.IsNotExist(err) {
						// If we are filtering by model and the state file doesn't exist, this session can't match.
						if modelFilter != "" {
							continue
						}
						// Otherwise, it's fine, just means no tokens/cost to add for this session.
					} else {
						return nil, fmt.Errorf("could not load agent state for session %s: %w", session.Name, err)
					}
				}
			}
		}

		// Apply model filter
		if modelFilter != "" {
			if agentState == nil || agentState.Model != modelFilter {
				continue
			}
		}

		// If we get here, the session matches all filters
		stats.TotalSessions++
		stats.StatusCounts[session.Status]++

		if agentState != nil {
			stats.TotalTokens += agentState.TokenUsage.TotalTokens
			stats.TotalPromptTokens += agentState.TokenUsage.TotalPromptTokens
			stats.TotalResponseTokens += agentState.TokenUsage.TotalResponseTokens
			stats.TotalCost += agent.CalculateCost(agentState.Model, agentState.TokenUsage)
		}
	}

	return stats, nil
}

func displayStats(stats *AggregateStats) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "AGGREGATE SESSION STATISTICS")
	fmt.Fprintln(w, "----------------------------")
	fmt.Fprintf(w, "Total Sessions (matching filters):\t%d\n", stats.TotalSessions)
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
	statsCmd.Flags().StringVar(&statusFilter, "status", "", "Filter by session status (e.g., completed, failed)")
	statsCmd.Flags().StringVar(&sinceFilter, "since", "", "Filter sessions started within a duration (e.g., 24h, 7d)")
	statsCmd.Flags().StringVar(&modelFilter, "model", "", "Filter by agent model name")
	rootCmd.AddCommand(statsCmd)
}
