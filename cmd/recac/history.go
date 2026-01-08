package main

import (
	"fmt"
	"os"
	"recac/internal/runner"
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
	Short: "List all sessions",
	Long:  `List all active and completed sessions.`,
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
			fmt.Fprintln(cmd.OutOrStdout(), "No sessions found.")
			return
		}

		// Sort sessions by start time
		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].StartTime.Before(sessions[j].StartTime)
		})

		activeSessions := []*runner.SessionState{}
		completedSessions := []*runner.SessionState{}

		for _, session := range sessions {
			if session.Status == "COMPLETED" {
				completedSessions = append(completedSessions, session)
			} else {
				activeSessions = append(activeSessions, session)
			}
		}
		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)

		if len(activeSessions) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Active Sessions:")
			fmt.Fprintln(w, "NAME\tSTATUS\tPID\tSTARTED\tWORKSPACE")
			for _, session := range activeSessions {
				started := session.StartTime.Format("2006-01-02 15:04:05")
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
					session.Name,
					session.Status,
					session.PID,
					started,
					session.Workspace,
				)
			}
			w.Flush()
			fmt.Fprintln(cmd.OutOrStdout())
		}

		if len(completedSessions) > 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "Completed Sessions:")
			fmt.Fprintln(w, "NAME\tSTATUS\tPID\tSTARTED\tENDED\tDURATION\tWORKSPACE")
			for _, session := range completedSessions {
				started := session.StartTime.Format("2006-01-02 15:04:05")
				ended := session.EndTime.Format("2006-01-02 15:04:05")
				duration := session.EndTime.Sub(session.StartTime).Round(time.Second)
				fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\t%s\t%s\n",
					session.Name,
					session.Status,
					session.PID,
					started,
					ended,
					duration,
					session.Workspace,
				)
			}
			w.Flush()
		}
	},
}
