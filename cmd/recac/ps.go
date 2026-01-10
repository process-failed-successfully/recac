package main

import (
	"fmt"
	"recac/internal/agent"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(psCmd)
	if psCmd.Flags().Lookup("costs") == nil {
		psCmd.Flags().BoolP("costs", "c", false, "Show token usage and cost information")
	}
}

var psCmd = &cobra.Command{
	Use:     "ps [session-name]",
	Aliases: []string{"list"},
	Short:   "List sessions or inspect a specific session",
	Long: `List all active and completed sessions.
If a session name is provided, it will display a comprehensive summary of that session.`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		// If a session name is provided, inspect it
		if len(args) == 1 {
			sessionName := args[0]
			session, err := sm.LoadSession(sessionName)
			if err != nil {
				return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
			}
			return DisplaySessionDetail(cmd, session, false)
		}

		// Otherwise, list all sessions
		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		if len(sessions) == 0 {
			cmd.Println("No sessions found.")
			return nil
		}

		showCosts, _ := cmd.Flags().GetBool("costs")

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
				agentState, err := loadAgentState(session.AgentStateFile)
				if err != nil {
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\tN/A\tN/A\tN/A\tN/A\n",
						session.Name,
						session.Status,
						started,
						duration,
					)
				} else {
					cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)
					fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%d\t%d\t%d\t$%.6f\n",
						session.Name,
						session.Status,
						started,
						duration,
						agentState.TokenUsage.TotalPromptTokens,
						agentState.TokenUsage.TotalResponseTokens,
						agentState.TokenUsage.TotalTokens,
						cost,
					)
				}
			} else {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
					session.Name,
					session.Status,
					started,
					duration,
				)
			}
		}

		return w.Flush()
	},
}
