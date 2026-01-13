package main

import (
	"fmt"
	"recac/internal/agent"
	"recac/internal/runner"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(nowCmd)
}

var nowCmd = &cobra.Command{
	Use:   "now",
	Short: "Provide a real-time snapshot of agent activity",
	Long: `The 'now' command offers an at-a-glance overview of the system's current state.
It shows all currently running agents and any agents that have completed in the last hour,
along with a focused summary of recent operational metrics.`,
	RunE: doNow,
}

func doNow(cmd *cobra.Command, args []string) error {
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

	// Filter sessions for "now" view
	oneHourAgo := time.Now().Add(-1 * time.Hour)
	var runningSessions, recentSessions []*runner.SessionState

	for _, s := range sessions {
		if s.Status == "running" {
			runningSessions = append(runningSessions, s)
		} else if s.EndTime.After(oneHourAgo) {
			recentSessions = append(recentSessions, s)
		}
	}

	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	defer w.Flush()

	// Display Running Sessions
	fmt.Fprintln(w, "🏃 Running Agents")
	fmt.Fprintln(w, "------------------")
	if len(runningSessions) == 0 {
		fmt.Fprintln(w, "No agents are currently running.\t\t\t")
	} else {
		fmt.Fprintln(w, "NAME\tMODEL\tUPTIME\tGIT SHA")
		sort.Slice(runningSessions, func(i, j int) bool {
			return runningSessions[i].StartTime.Before(runningSessions[j].StartTime)
		})
		for _, s := range runningSessions {
			uptime := time.Since(s.StartTime).Round(time.Second)
			model := getModelFromState(s)
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, model, uptime, s.StartCommitSHA)
		}
	}
	w.Flush()

	// Display Recently Completed Sessions
	fmt.Fprintln(w, "\n✅ Recently Completed (Last Hour)")
	fmt.Fprintln(w, "---------------------------------")
	if len(recentSessions) == 0 {
		fmt.Fprintln(w, "No agents completed in the last hour.\t\t\t")
	} else {
		fmt.Fprintln(w, "NAME\tSTATUS\tDURATION\tCOST")
		sort.Slice(recentSessions, func(i, j int) bool {
			return recentSessions[i].EndTime.After(recentSessions[j].EndTime)
		})
		for _, s := range recentSessions {
			duration := s.EndTime.Sub(s.StartTime).Round(time.Second)
			cost, _ := getCostFromState(s)
			fmt.Fprintf(w, "%s\t%s\t%s\t$%.4f\n", s.Name, s.Status, duration, cost)
		}
	}
	w.Flush()

	// Display Summary for the Last Hour
	calculateAndDisplayRecentSummary(cmd, w, sessions, oneHourAgo)

	return nil
}

func calculateAndDisplayRecentSummary(cmd *cobra.Command, w *tabwriter.Writer, sessions []*runner.SessionState, since time.Time) {
	var recentTotal, recentCompleted, recentErrored int
	var recentCost float64

	for _, s := range sessions {
		if s.StartTime.After(since) {
			recentTotal++
			if s.Status == "completed" {
				recentCompleted++
			} else if s.Status == "error" {
				recentErrored++
			}
			cost, _ := getCostFromState(s)
			recentCost += cost
		}
	}

	fmt.Fprintln(w, "\n📈 Summary (Last Hour)")
	fmt.Fprintln(w, "-----------------------")
	fmt.Fprintf(w, "Sessions Started:\t%d\n", recentTotal)
	if recentTotal > 0 {
		successRate := float64(recentCompleted) / float64(recentCompleted+recentErrored) * 100
		if recentCompleted+recentErrored == 0 { // Avoid NaN if only running sessions
			successRate = 100
		}
		fmt.Fprintf(w, "Success Rate:\t%.1f%%\n", successRate)
	} else {
		fmt.Fprintln(w, "Success Rate:\tN/A")

	}
	fmt.Fprintf(w, "Est. Cost:\t$%.4f\n", recentCost)
}

// Helper to safely get model from agent state
func getModelFromState(s *runner.SessionState) string {
	if s.AgentStateFile != "" {
		state, err := loadAgentState(s.AgentStateFile)
		if err == nil && state != nil {
			return state.Model
		}
	}
	return "N/A"
}

// Helper to safely get cost from agent state
func getCostFromState(s *runner.SessionState) (float64, int) {
	if s.AgentStateFile != "" {
		state, err := loadAgentState(s.AgentStateFile)
		if err == nil && state != nil {
			cost := agent.CalculateCost(state.Model, state.TokenUsage)
			tokens := state.TokenUsage.TotalTokens
			return cost, tokens
		}
	}
	return 0.0, 0
}
