package main

import (
	"fmt"
	"recac/internal/runner"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

var (
	findNameFlag   string
	findStatusFlag string
)

func init() {
	rootCmd.AddCommand(findCmd)
	findCmd.Flags().StringVar(&findNameFlag, "name", "", "Filter sessions by name (substring match)")
	findCmd.Flags().StringVar(&findStatusFlag, "status", "", "Filter sessions by status (e.g., running, error, completed)")
}

var findCmd = &cobra.Command{
	Use:   "find",
	Short: "Find sessions by name or status",
	Long:  `Find sessions by filtering on their name or status.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		allSessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		var filteredSessions []*runner.SessionState
		for _, s := range allSessions {
			nameMatch := findNameFlag == "" || strings.Contains(s.Name, findNameFlag)
			statusMatch := findStatusFlag == "" || strings.EqualFold(s.Status, findStatusFlag)

			if nameMatch && statusMatch {
				filteredSessions = append(filteredSessions, s)
			}
		}

		if len(filteredSessions) == 0 {
			cmd.Println("No matching sessions found.")
			return nil
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tSTARTED\tDURATION")
		for _, session := range filteredSessions {
			started := session.StartTime.Format("2006-01-02 15:04:05")
			var duration string
			if session.EndTime.IsZero() {
				duration = time.Since(session.StartTime).Round(time.Second).String()
			} else {
				duration = session.EndTime.Sub(session.StartTime).Round(time.Second).String()
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				session.Name, session.Status, started, duration)
		}
		return w.Flush()
	},
}
