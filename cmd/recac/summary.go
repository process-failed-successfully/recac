package main

import (
	"fmt"
	"recac/internal/agent"
	"recac/internal/ui"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	summaryCmd.Flags().BoolP("watch", "w", false, "Enable real-time dashboard view")
	rootCmd.AddCommand(summaryCmd)
}

var summaryCmd = &cobra.Command{
	Use:   "summary",
	Short: "Provide a high-level summary of all sessions",
	Long: `The summary command gives a dashboard view of agent activity,
including aggregate statistics, recent sessions, and costliest sessions.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		watch, _ := cmd.Flags().GetBool("watch")
		if watch {
			return ui.StartSummaryDashboard()
		}
		return doSummary(cmd)
	},
}

func doSummary(cmd *cobra.Command) error {
	sm, err := sessionManagerFactory()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		cmd.Println("No sessions found.")
		return nil
	}

	// Aggregate Stats
	var totalTokens int
	var totalCost float64
	completed, errored, running := 0, 0, 0

	sessionCosts := make(map[string]float64)

	for _, s := range sessions {
		switch s.Status {
		case "completed":
			completed++
		case "error":
			errored++
		case "running":
			running++
		}

		if s.AgentStateFile != "" {
			state, err := loadAgentState(s.AgentStateFile)
			if err == nil && state != nil {
				cost := agent.CalculateCost(state.Model, state.TokenUsage)
				totalCost += cost
				totalTokens += state.TokenUsage.TotalTokens
				sessionCosts[s.Name] = cost
			}
		}
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)

	fmt.Fprintln(w, "📊 Aggregate Stats")
	fmt.Fprintln(w, "-------------------")
	fmt.Fprintf(w, "Total Sessions:\t%d\n", len(sessions))
	fmt.Fprintf(w, "Completed:\t%d\n", completed)
	fmt.Fprintf(w, "Errored:\t%d\n", errored)
	fmt.Fprintf(w, "Running:\t%d\n", running)
	if len(sessions) > 0 {
		successRate := float64(completed) / float64(len(sessions)) * 100
		fmt.Fprintf(w, "Success Rate:\t%.2f%%\n", successRate)
	}
	fmt.Fprintf(w, "Total Tokens:\t%d\n", totalTokens)
	fmt.Fprintf(w, "Total Est. Cost:\t$%.4f\n", totalCost)
	w.Flush()

	// Recent Sessions
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	fmt.Fprintln(w, "\n🕒 Recent Sessions (Top 5)")
	fmt.Fprintln(w, "-------------------------")
	fmt.Fprintln(w, "NAME\tSTATUS\tSTART TIME\tDURATION")
	for i, s := range sessions {
		if i >= 5 {
			break
		}
		duration := time.Since(s.StartTime).Round(time.Second)
		if !s.EndTime.IsZero() {
			duration = s.EndTime.Sub(s.StartTime).Round(time.Second)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, s.Status, s.StartTime.Format(time.RFC3339), duration)
	}
	w.Flush()

	// Most Expensive Sessions
	sort.Slice(sessions, func(i, j int) bool {
		return sessionCosts[sessions[i].Name] > sessionCosts[sessions[j].Name]
	})

	fmt.Fprintln(w, "\n💰 Most Expensive Sessions (Top 5)")
	fmt.Fprintln(w, "---------------------------------")
	fmt.Fprintln(w, "NAME\tCOST\tTOKENS\tMODEL")
	for i, s := range sessions {
		if i >= 5 {
			break
		}
		cost := sessionCosts[s.Name]
		if cost == 0 {
			continue // Don't show sessions with no cost
		}

		tokens := 0
		model := "N/A"
		if s.AgentStateFile != "" {
			state, err := loadAgentState(s.AgentStateFile)
			if err == nil && state != nil {
				tokens = state.TokenUsage.TotalTokens
				model = state.Model
			}
		}
		fmt.Fprintf(w, "%s\t$%.4f\t%d\t%s\n", s.Name, cost, tokens, model)
	}
	w.Flush()

	return nil
}
