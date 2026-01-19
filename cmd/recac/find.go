package main

import (
	"fmt"
	"recac/internal/runner"
	"regexp"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(findCmd)
	findCmd.Flags().String("status", "", "Filter sessions by status (e.g., 'running', 'completed', 'error')")
	findCmd.Flags().String("since", "", "Filter sessions started after a specific duration (e.g., '1h', '30m') or timestamp ('2006-01-02')")
	findCmd.Flags().String("before", "", "Filter sessions started before a specific duration (e.g., '1h', '30m') or timestamp ('2006-01-02')")
	findCmd.Flags().String("goal", "", "Filter sessions by a keyword in their initial goal or command arguments")
	findCmd.Flags().String("file", "", "Filter sessions by files they modified (using regex on git diff)")
	findCmd.Flags().String("error", "", "Filter sessions by a keyword in their error message")
}

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "Find sessions based on rich criteria",
	Long: `Find sessions by filtering on metadata, goals, files modified, or errors.
This command provides a powerful way to search through the history of agent activity
to debug issues, analyze performance, or find specific examples of agent work.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		// --- FILTERING LOGIC ---
		statusFilter, _ := cmd.Flags().GetString("status")
		sinceFilter, _ := cmd.Flags().GetString("since")
		beforeFilter, _ := cmd.Flags().GetString("before")
		goalFilter, _ := cmd.Flags().GetString("goal")
		fileFilter, _ := cmd.Flags().GetString("file")
		errorFilter, _ := cmd.Flags().GetString("error")

		var filteredSessions []*runner.SessionState

		for _, s := range sessions {
			// Status filter
			if statusFilter != "" && !strings.EqualFold(s.Status, statusFilter) {
				continue
			}

			// Since filter
			if sinceFilter != "" {
				sinceTime, err := parseTimeFilter(sinceFilter)
				if err != nil {
					return err
				}
				if s.StartTime.Before(sinceTime) {
					continue
				}
			}

			// Before filter
			if beforeFilter != "" {
				beforeTime, err := parseTimeFilter(beforeFilter)
				if err != nil {
					return err
				}
				if s.StartTime.After(beforeTime) {
					continue
				}
			}

			// Goal filter
			if goalFilter != "" && !strings.Contains(strings.Join(s.Command, " "), goalFilter) {
				continue
			}

			// Error filter
			if errorFilter != "" && !strings.Contains(s.Error, errorFilter) {
				continue
			}

			// File filter (regex)
			if fileFilter != "" {
				diff, err := sm.GetSessionGitDiffStat(s.Name)
				if err != nil {
					// Ignore sessions where we can't get a diff
					continue
				}
				matched, err := regexp.MatchString(fileFilter, diff)
				if err != nil {
					return fmt.Errorf("invalid regex for --file: %w", err)
				}
				if !matched {
					continue
				}
			}

			filteredSessions = append(filteredSessions, s)
		}
		sessions = filteredSessions

		if len(sessions) == 0 {
			cmd.Println("No sessions found matching the criteria.")
			return nil
		}

		sort.SliceStable(sessions, func(i, j int) bool {
			return sessions[i].StartTime.After(sessions[j].StartTime)
		})

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "SESSION ID\tSTATUS\tGOAL\tSTARTED\tDURATION\tERROR")

		for _, s := range sessions {
			started := s.StartTime.Format("2006-01-02 15:04:05")
			var duration string
			if s.EndTime.IsZero() {
				duration = time.Since(s.StartTime).Round(time.Second).String()
			} else {
				duration = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
			}

			// Truncate goal and error for display
			goal := strings.Join(s.Command, " ")
			if len(goal) > 50 {
				goal = goal[:47] + "..."
			}
			errorMessage := s.Error
			if len(errorMessage) > 40 {
				errorMessage = errorMessage[:37] + "..."
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				s.Name, s.Status, goal, started, duration, errorMessage)
		}

		return w.Flush()
	},
}
