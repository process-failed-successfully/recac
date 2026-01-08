package main

import (
	"fmt"
	"os"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(historyCmd)
}

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show session history",
	Long:  `Show a detailed history of all past and present sessions, including start time, end time, and duration.`,
	Run: func(cmd *cobra.Command, args []string) {
		sm, err := newSessionManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create session manager: %v\n", err)
			exit(1)
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to list sessions: %v\n", err)
			exit(1)
		}

		if len(sessions) == 0 {
			fmt.Println("No session history found.")
			return
		}

		// Sort sessions by StartTime
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].StartTime.Before(sessions[j].StartTime)
		})

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "NAME\tSTATUS\tSTART TIME\tEND TIME\tDURATION\tWORKSPACE")
		fmt.Fprintln(w, "----\t------\t----------\t--------\t--------\t---------")

		for _, session := range sessions {
			startTime := session.StartTime.Format("2006-01-02 15:04:05")
			endTime := "N/A"
			duration := "N/A"

			if !session.EndTime.IsZero() {
				endTime = session.EndTime.Format("2006-01-02 15:04:05")
				duration = session.EndTime.Sub(session.StartTime).Round(time.Second).String()
			} else if session.Status == "running" {
				duration = time.Since(session.StartTime).Round(time.Second).String()
			}

			fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
				session.Name,
				session.Status,
				startTime,
				endTime,
				duration,
				session.Workspace,
			)
		}
		w.Flush()
	},
}
