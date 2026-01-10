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
	rootCmd.AddCommand(topCmd)
	topCmd.Flags().IntP("number", "n", 10, "Number of sessions to display")
}

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "List the top N most expensive sessions",
	Long:  `Calculates the cost for each session and displays a sorted list of the most expensive ones.`,
	RunE: func(cmd *cobra.Command, args []string) error {
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

		type sessionWithCost struct {
			*runner.SessionState
			cost float64
		}

		var sessionsWithCosts []sessionWithCost

		for _, session := range sessions {
			agentState, err := loadAgentState(session.AgentStateFile)
			// Only include sessions where we can calculate a cost
			if err == nil && agentState.Model != "" {
				cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)
				sessionsWithCosts = append(sessionsWithCosts, sessionWithCost{SessionState: session, cost: cost})
			}
		}

		// Sort sessions by cost in descending order
		sort.Slice(sessionsWithCosts, func(i, j int) bool {
			return sessionsWithCosts[i].cost > sessionsWithCosts[j].cost
		})

		number, _ := cmd.Flags().GetInt("number")
		if number > 0 && len(sessionsWithCosts) > number {
			sessionsWithCosts = sessionsWithCosts[:number]
		}

		if len(sessionsWithCosts) == 0 {
			cmd.Println("No sessions with cost data found.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tCOST\tSTARTED\tDURATION")

		for _, s := range sessionsWithCosts {
			started := s.StartTime.Format("2006-01-02 15:04:05")
			var duration string
			if s.EndTime.IsZero() {
				duration = time.Since(s.StartTime).Round(time.Second).String()
			} else {
				duration = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
			}

			fmt.Fprintf(w, "%s\t%s\t$%.6f\t%s\t%s\n",
				s.Name, s.Status, s.cost, started, duration)
		}

		return w.Flush()
	},
}
