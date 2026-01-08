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

var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Show the history of all recac sessions",
	Long:  `Displays a list of all past and present recac sessions, including their status, start time, and other relevant metadata.`,
	RunE:  runHistory,
}

func init() {
	rootCmd.AddCommand(historyCmd)
}

func runHistory(cmd *cobra.Command, args []string) error {
	manager, err := runner.NewSessionManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	sessions, err := manager.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No session history found.")
		return nil
	}

	// Sort sessions by start time, descending
	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].StartTime.After(sessions[j].StartTime)
	})

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	defer w.Flush()

	fmt.Fprintln(w, "SESSION ID\tSTATUS\tSTART TIME\tPID\tWORKSPACE")
	fmt.Fprintln(w, "----------\t------\t----------\t---\t---------")

	for _, session := range sessions {
		startTime := session.StartTime.Format(time.RFC3339)
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\n", session.Name, session.Status, startTime, session.PID, session.Workspace)
	}

	return nil
}
