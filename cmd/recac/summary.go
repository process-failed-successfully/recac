package main

import (
	"fmt"
	"sort"
	"text/tabwriter"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(summaryCmd)
}

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Display a high-level summary of all sessions",
	Long:  `Provides a dashboard view of the most important statistics, including running sessions, costs, and recent activity.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("could not create session manager: %w", err)
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("could not list sessions: %w", err)
		}

		if len(sessions) == 0 {
			cmd.Println("No sessions found to summarize.")
			return nil
		}

		displaySummary(cmd, sessions)

		return nil
	},
}

// SummaryStats holds all the aggregated data for the summary view.
type SummaryStats struct {
	TotalSessions     int
	RunningSessions   int
	ErroredSessions   int
	TotalCost         float64
	TotalTokens       int
	TopSessionsByCost []SessionWithCost
	RecentSessions    []*runner.SessionState
}

// SessionWithCost pairs a session with its calculated cost.
type SessionWithCost struct {
	*runner.SessionState
	Cost float64
}

func analyzeSessionsForSummary(sessions []*runner.SessionState) (*SummaryStats, error) {
	stats := &SummaryStats{
		TotalSessions: len(sessions),
	}

	sessionsWithCosts := make([]SessionWithCost, 0, len(sessions))

	for _, s := range sessions {
		if s.Status == "RUNNING" {
			stats.RunningSessions++
		}
		if s.Status == "ERROR" {
			stats.ErroredSessions++
		}

		cost := 0.0
		if s.AgentStateFile != "" {
			agentState, err := loadAgentState(s.AgentStateFile)
			if err == nil {
				cost = agent.CalculateCost(agentState.Model, agentState.TokenUsage)
				stats.TotalCost += cost
				stats.TotalTokens += agentState.TokenUsage.TotalTokens
			}
		}
		sessionsWithCosts = append(sessionsWithCosts, SessionWithCost{SessionState: s, Cost: cost})
	}

	// Sort by cost for top sessions
	sort.Slice(sessionsWithCosts, func(i, j int) bool {
		return sessionsWithCosts[i].Cost > sessionsWithCosts[j].Cost
	})
	stats.TopSessionsByCost = sessionsWithCosts
	if len(stats.TopSessionsByCost) > 5 {
		stats.TopSessionsByCost = stats.TopSessionsByCost[:5]
	}

	// Create a copy for time sorting
	sessionsForTimeSort := make([]*runner.SessionState, len(sessions))
	copy(sessionsForTimeSort, sessions)

	// Sort by time for recent sessions
	sort.Slice(sessionsForTimeSort, func(i, j int) bool {
		return sessionsForTimeSort[i].StartTime.After(sessionsForTimeSort[j].StartTime)
	})
	stats.RecentSessions = sessionsForTimeSort
	if len(stats.RecentSessions) > 5 {
		stats.RecentSessions = stats.RecentSessions[:5]
	}

	return stats, nil
}

func displaySummary(cmd *cobra.Command, sessions []*runner.SessionState) {
	stats, err := analyzeSessionsForSummary(sessions)
	if err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error analyzing sessions: %v\n", err)
		return
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)

	// --- Overall Stats ---
	fmt.Fprintln(w, "OVERALL STATS")
	fmt.Fprintln(w, "-------------")
	fmt.Fprintf(w, "Total Sessions:\t%d\n", stats.TotalSessions)
	fmt.Fprintf(w, "Running:\t%d\n", stats.RunningSessions)
	fmt.Fprintf(w, "Errored:\t%d\n", stats.ErroredSessions)
	fmt.Fprintln(w)

	// --- Cost Analysis ---
	fmt.Fprintln(w, "COST ANALYSIS")
	fmt.Fprintln(w, "-------------")
	fmt.Fprintf(w, "Total Estimated Cost:\t$%.4f\n", stats.TotalCost)
	fmt.Fprintf(w, "Total Tokens:\t%d\n", stats.TotalTokens)
	fmt.Fprintln(w)

	// --- Top 5 Sessions by Cost ---
	fmt.Fprintln(w, "TOP 5 SESSIONS BY COST")
	fmt.Fprintln(w, "----------------------")
	fmt.Fprintln(w, "NAME\tSTATUS\tCOST")
	for _, s := range stats.TopSessionsByCost {
		costStr := "N/A"
		if s.Cost > 0 {
			costStr = fmt.Sprintf("$%.6f", s.Cost)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Status, costStr)
	}
	fmt.Fprintln(w)

	// --- 5 Most Recent Sessions ---
	fmt.Fprintln(w, "5 MOST RECENT SESSIONS")
	fmt.Fprintln(w, "----------------------")
	fmt.Fprintln(w, "NAME\tSTATUS\tSTARTED\tDURATION")
	for _, s := range stats.RecentSessions {
		started := s.StartTime.Format("2006-01-02 15:04:05")
		var duration string
		if s.EndTime.IsZero() {
			duration = time.Since(s.StartTime).Round(time.Second).String()
		} else {
			duration = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, s.Status, started, duration)
	}

	w.Flush()
}
