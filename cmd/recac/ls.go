package main

import (
	"fmt"
	"recac/internal/utils"
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
	lsCmd.Flags().String("since", "", "Filter sessions started after a specific duration (e.g., '1h', '30m') or timestamp ('2006-01-02')")
	lsCmd.Flags().String("before", "", "Filter sessions started before a specific duration (e.g., '1h', '30m') or timestamp ('2006-01-02')")
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
		sinceFilter, _ := cmd.Flags().GetString("since")
		beforeFilter, _ := cmd.Flags().GetString("before")

		// Apply status filter
		if statusFilter != "" {
			var filteredSessions []*runner.SessionState
			for _, s := range sessions {
				if strings.EqualFold(s.Status, statusFilter) {
					filteredSessions = append(filteredSessions, s)
				}
			}
			sessions = filteredSessions
		}

		// Apply 'since' time filter
		if sinceFilter != "" {
			sinceTime, err := parseTimeFilter(sinceFilter)
			if err != nil {
				return err
			}
			var filteredSessions []*runner.SessionState
			for _, s := range sessions {
				if s.StartTime.After(sinceTime) {
					filteredSessions = append(filteredSessions, s)
				}
			}
			sessions = filteredSessions
		}

		// Apply 'before' time filter
		if beforeFilter != "" {
			beforeTime, err := parseTimeFilter(beforeFilter)
			if err != nil {
				return err
			}
			var filteredSessions []*runner.SessionState
			for _, s := range sessions {
				if s.StartTime.Before(beforeTime) {
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

// parseTimeFilter parses a string that can be either a duration (including days) or a timestamp.
func parseTimeFilter(value string) (time.Time, error) {
	// Try parsing as a duration first (e.g., "7d", "24h")
	duration, err := utils.ParseStaleDuration(value)
	if err == nil {
		return time.Now().Add(-duration), nil
	}

	// If not a duration, try parsing as a timestamp
	layouts := []string{time.RFC3339, "2006-01-02"}
	for _, layout := range layouts {
		t, err := time.Parse(layout, value)
		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("invalid time value %q: must be a duration (e.g., '1h') or a timestamp (e.g., '2006-01-02')", value)
}
