package main

import (
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(psCmd)
}

var psCmd = &cobra.Command{
	Use:     "ps",
	Aliases: []string{"list"},
	Short:   "List all sessions",
	Long:    `List all active and completed sessions.`,
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

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tSTARTED\tDURATION")

		for _, session := range sessions {
			started := session.StartTime.Format("2006-01-02 15:04:05")
			var duration string
			if session.EndTime.IsZero() {
				duration = time.Since(session.StartTime).Round(time.Second).String()
			} else {
				duration = session.EndTime.Sub(session.StartTime).Round(time.Second).String()
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				session.Name,
				session.Status,
				started,
				duration,
			)
		}

		return w.Flush()
	},
}
