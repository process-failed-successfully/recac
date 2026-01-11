package main

import (
	"fmt"
	"recac/internal/agent"
	"recac/internal/runner"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(psCmd)
	if psCmd.Flags().Lookup("costs") == nil {
		psCmd.Flags().BoolP("costs", "c", false, "Show token usage and cost information")
	}
	if psCmd.Flags().Lookup("sort") == nil {
		psCmd.Flags().String("sort", "time", "Sort sessions by 'cost', 'time', or 'name'")
	}
	if psCmd.Flags().Lookup("errors") == nil {
		psCmd.Flags().BoolP("errors", "e", false, "Show the first line of the error for failed sessions")
	}
	if psCmd.Flags().Lookup("status") == nil {
		psCmd.Flags().String("status", "", "Filter sessions by status (e.g., 'running', 'completed', 'error')")
	}
}

var psCmd = &cobra.Command{
	Use:     "ps [session-name]",
	Aliases: []string{"list"},
	Short:   "List sessions or display details for a specific session",
	Long: `List all active and completed sessions.
If a session name is provided, it displays a detailed summary for that session.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// If a session name is provided, show details for that session.
		if len(args) > 0 {
			sessionName := args[0]
			session, err := sm.LoadSession(sessionName)
			if err != nil {
				return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
			}
			// The --costs flag is not applicable here, so we pass false for fullLogs.
			// DisplaySessionDetail shows cost info by default.
			return DisplaySessionDetail(cmd, session, false)
		}

		// Otherwise, list all sessions.
		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		// Handle status filtering
		statusFilter, _ := cmd.Flags().GetString("status")
		if statusFilter != "" {
			var filteredSessions []*runner.SessionState
			for _, s := range sessions {
				if strings.EqualFold(s.Status, statusFilter) {
					filteredSessions = append(filteredSessions, s)
				}
			}
			sessions = filteredSessions
		}

		if len(sessions) == 0 {
			cmd.Println("No sessions found.")
			return nil
		}

		showCosts, _ := cmd.Flags().GetBool("costs")
		sortBy, _ := cmd.Flags().GetString("sort")

		// Pre-calculate costs for sorting if needed
		sessionCosts := make(map[string]float64)
		if sortBy == "cost" {
			for _, session := range sessions {
				agentState, err := loadAgentState(session.AgentStateFile)
				if err == nil {
					sessionCosts[session.Name] = agent.CalculateCost(agentState.Model, agentState.TokenUsage)
				}
			}
		}

		// Sort sessions
		sort.SliceStable(sessions, func(i, j int) bool {
			switch sortBy {
			case "cost":
				// Handle cases where cost is not available
				costI, okI := sessionCosts[sessions[i].Name]
				costJ, okJ := sessionCosts[sessions[j].Name]
				if okI && okJ {
					return costI > costJ // Higher cost first
				}
				return okI // Sessions with cost come before those without
			case "name":
				return sessions[i].Name < sessions[j].Name
			case "time":
				fallthrough // Default to sorting by time
			default:
				return sessions[i].StartTime.After(sessions[j].StartTime)
			}
		})

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		if showCosts {
			fmt.Fprintln(w, "NAME\tSTATUS\tSTARTED\tDURATION\tPROMPT_TOKENS\tCOMPLETION_TOKENS\tTOTAL_TOKENS\tCOST")
		} else {
			fmt.Fprintln(w, "NAME\tSTATUS\tSTARTED\tDURATION")
		}

		for _, session := range sessions {
			started := session.StartTime.Format("2006-01-02 15:04:05")
			var duration string
			if session.EndTime.IsZero() {
				duration = time.Since(session.StartTime).Round(time.Second).String()
			} else {
				duration = session.EndTime.Sub(session.StartTime).Round(time.Second).String()
			}

			if showCosts {
				cost, hasCost := sessionCosts[session.Name]
				// If costs were not pre-calculated, calculate them now
				if !hasCost && sortBy != "cost" {
					agentState, err := loadAgentState(session.AgentStateFile)
					if err == nil {
						cost = agent.CalculateCost(agentState.Model, agentState.TokenUsage)
						hasCost = true
					}
				}

				if !hasCost {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\tN/A\tN/A\tN/A\tN/A\n",
						session.Name, session.Status, started, duration)
				} else {
					// We need to reload agentState to get token counts
					agentState, _ := loadAgentState(session.AgentStateFile)
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t$%.6f\n",
						session.Name, session.Status, started, duration,
						agentState.TokenUsage.TotalPromptTokens,
						agentState.TokenUsage.TotalResponseTokens,
						agentState.TokenUsage.TotalTokens,
						cost,
					)
				}
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					session.Name, session.Status, started, duration)
			}
		}

		return w.Flush()
	},
}
