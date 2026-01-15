package main

import (
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"recac/internal/runner"
	"recac/internal/utils"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(lsCmd)
	lsCmd.Flags().String("status", "", "Filter sessions by status (e.g., 'running', 'completed', 'error')")
	lsCmd.Flags().String("since", "", "Filter sessions started after a specific duration (e.g., '1h', '30m') or timestamp ('2006-01-02')")
	lsCmd.Flags().String("stale", "", "Filter sessions that have been inactive for a given duration (e.g., '7d', '24h')")
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

		// --- Filtering ---
		statusFilter, _ := cmd.Flags().GetString("status")
		sinceFilter, _ := cmd.Flags().GetString("since")
		staleFilter, _ := cmd.Flags().GetString("stale")

		// Stale Time Calculation
		var staleTime time.Time
		if staleFilter != "" {
			duration, err := utils.ParseStaleDuration(staleFilter)
			if err != nil {
				return fmt.Errorf("invalid 'stale' value %q: %w", staleFilter, err)
			}
			staleTime = time.Now().Add(-duration)
		}

		// Since Time Calculation
		var sinceTime time.Time
		if sinceFilter != "" {
			duration, err := time.ParseDuration(sinceFilter)
			if err == nil {
				sinceTime = time.Now().Add(-duration)
			} else {
				layouts := []string{time.RFC3339, "2006-01-02"}
				parsed := false
				for _, layout := range layouts {
					t, err := time.Parse(layout, sinceFilter)
					if err == nil {
						sinceTime = t
						parsed = true
						break
					}
				}
				if !parsed {
					return fmt.Errorf("invalid 'since' value %q: must be a duration or timestamp", sinceFilter)
				}
			}
		}

		// Apply filters in a single loop
		if statusFilter != "" || sinceFilter != "" || staleFilter != "" {
			var filteredSessions []*runner.SessionState
			for _, s := range sessions {
				// Status filter
				if statusFilter != "" && !strings.EqualFold(s.Status, statusFilter) {
					continue
				}

				// Since filter
				if !sinceTime.IsZero() && s.StartTime.Before(sinceTime) {
					continue
				}

				// Stale filter: if the flag is present, a session must be stale to be included.
				if !staleTime.IsZero() {
					// For `ls`, we use EndTime (or StartTime if the session is not finished) as a proxy for activity,
					// to avoid the overhead of reading the agent state file like `ps` does.
					activityTime := s.EndTime
					if activityTime.IsZero() {
						activityTime = s.StartTime
					}
					// If the session is NOT stale, skip it.
					if activityTime.IsZero() || !activityTime.Before(staleTime) {
						continue
					}
				}

				filteredSessions = append(filteredSessions, s)
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
