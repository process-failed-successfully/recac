package main

import (
	"fmt"
	"io"
	"sort"
	"strings"
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
	Short: "Display a summary of all sessions",
	Long:  "Provides a high-level dashboard of session statistics, usage, and cost.",
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}
		return doSummary(cmd.OutOrStdout(), sm)
	},
}

// sessionWithCost is a helper struct to sort sessions by their cost.
type sessionWithCost struct {
	*runner.SessionState
	Cost float64
}

func doSummary(out io.Writer, sm ISessionManager) error {
	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Fprintln(out, "No sessions to display.")
		return nil
	}

	totalTokens := 0
	totalCost := 0.0
	statusCounts := make(map[string]int)
	sessionsWithCosts := make([]sessionWithCost, 0, len(sessions))

	for _, session := range sessions {
		statusCounts[session.Status]++
		cost := 0.0
		if state, err := loadAgentState(session.AgentStateFile); err == nil {
			totalTokens += state.TokenUsage.TotalTokens
			cost = agent.CalculateCost(state.Model, state.TokenUsage)
			totalCost += cost
		}
		sessionsWithCosts = append(sessionsWithCosts, sessionWithCost{SessionState: session, Cost: cost})
	}

	fmt.Fprintln(out, "==> Overview")
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintf(w, "Total Sessions:\t%d\n", len(sessions))
	fmt.Fprintf(w, "  - Running:\t%d\n", statusCounts["running"])
	fmt.Fprintf(w, "  - Completed:\t%d\n", statusCounts["completed"])
	fmt.Fprintf(w, "  - Errored:\t%d\n", statusCounts["error"])
	w.Flush()
	fmt.Fprintln(out)

	fmt.Fprintln(out, "==> Usage Stats (All Time)")
	fmt.Fprintf(w, "Total Tokens:\t%d\n", totalTokens)
	fmt.Fprintf(w, "Total Cost:\t$%.2f\n", totalCost)
	w.Flush()
	fmt.Fprintln(out)

	// Sort sessions by start time for "Recent Sessions"
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	fmt.Fprintln(out, "==> Recent Sessions (Last 5)")
	printSessionTable(out, sessions, sessionsWithCosts, 5, true)
	fmt.Fprintln(out)

	// Sort sessions by cost for "Most Expensive Sessions"
	sort.SliceStable(sessionsWithCosts, func(i, j int) bool {
		return sessionsWithCosts[i].Cost > sessionsWithCosts[j].Cost
	})

	fmt.Fprintln(out, "==> Most Expensive Sessions (Top 3)")
	// We need to convert sessionsWithCosts back to a list of sessions for printing
	expensiveSessions := make([]*runner.SessionState, len(sessionsWithCosts))
	for i, swc := range sessionsWithCosts {
		expensiveSessions[i] = swc.SessionState
	}
	printSessionTable(out, expensiveSessions, sessionsWithCosts, 3, false)

	return nil
}

func getSessionCost(session *runner.SessionState, sessionsWithCosts []sessionWithCost) float64 {
	for _, swc := range sessionsWithCosts {
		if swc.Name == session.Name {
			return swc.Cost
		}
	}
	return 0.0
}

func printSessionTable(out io.Writer, sessions []*runner.SessionState, sessionsWithCosts []sessionWithCost, limit int, showAge bool) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	header := "NAME\tSTATUS\tCOST"
	if showAge {
		header = "NAME\tSTATUS\tAGE\tCOST"
	}
	fmt.Fprintln(w, header)

	if limit > len(sessions) {
		limit = len(sessions)
	}

	for _, s := range sessions[:limit] {
		cost := getSessionCost(s, sessionsWithCosts)
		costStr := fmt.Sprintf("$%.2f", cost)

		if showAge {
			age := time.Since(s.StartTime).Round(time.Minute).String()
			// a bit of a hack to make the output cleaner e.g. "5m" instead of "5m0s"
			cleanAge := strings.Replace(age, "m0s", "m", 1)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, s.Status, cleanAge, costStr)
		} else {
			fmt.Fprintf(w, "%s\t%s\t%s\n", s.Name, s.Status, costStr)
		}
	}
	w.Flush()
}
