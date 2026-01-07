package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions (deprecated, use 'status')",
	Long:  `List all active and completed sessions.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stderr, "Warning: the \"list\" command is deprecated and will be removed in a future release. Please use \"recac status\" instead.")

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
			fmt.Println("No sessions found.")
			return
		}

		fmt.Println("Active Sessions:")
		fmt.Println("===============")
		fmt.Printf("%-20s %-10s %-10s %-20s %s\n", "NAME", "STATUS", "PID", "STARTED", "WORKSPACE")
		fmt.Println("--------------------------------------------------------------------------------")

		for _, session := range sessions {
			started := session.StartTime.Format("2006-01-02 15:04:05")
			fmt.Printf("%-20s %-10s %-10d %-20s %s\n",
				session.Name,
				session.Status,
				session.PID,
				started,
				session.Workspace,
			)
		}
	},
}
