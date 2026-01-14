package main

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/runner"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(lsCmd)
	lsCmd.Flags().String("status", "", "Filter sessions by status (e.g., 'running', 'completed', 'error')")
}

var lsCmd = &cobra.Command{
	Use:     "ls",
	Short:   "List sessions in a simple, script-friendly format",
	Long:    `List all active and completed local sessions in a format suitable for scripting and piping.`,
	Args:    cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

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

		sort.SliceStable(sessions, func(i, j int) bool {
			return sessions[i].StartTime.After(sessions[j].StartTime)
		})

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "SESSION_ID\tSTATUS\tSTARTED\tDURATION")

		for _, s := range sessions {
			started := s.StartTime.Format("2006-01-02 15:04:05")
			var duration string
			if s.EndTime.IsZero() {
				duration = time.Since(s.StartTime).Round(time.Second).String()
			} else {
				duration = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				s.Name, s.Status, started, duration)
		}

		return w.Flush()
	},
}
