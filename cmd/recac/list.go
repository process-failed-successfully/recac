package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"recac/internal/runner"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

func init() {
	listCmd.Flags().String("status", "", "Filter sessions by status (e.g., running, completed, error)")
	listCmd.Flags().String("sort-by", "start_time", "Sort sessions by attribute (name, start_time)")
	listCmd.Flags().Bool("json", false, "Output in JSON format")
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all sessions",
	Long:  `List all active and completed sessions, with optional filtering and sorting.`,
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

		status, _ := cmd.Flags().GetString("status")
		sortOpt, _ := cmd.Flags().GetString("sort-by")
		jsonOpt, _ := cmd.Flags().GetBool("json")

		filteredSessions := filterSessions(sessions, status)
		sortSessions(filteredSessions, sortOpt)

		if len(filteredSessions) == 0 {
			if jsonOpt {
				fmt.Fprintln(cmd.OutOrStdout(), "[]")
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "No sessions found.")
			}
			return
		}

		if jsonOpt {
			printSessionsJSON(cmd.OutOrStdout(), filteredSessions)
		} else {
			printSessions(cmd.OutOrStdout(), filteredSessions)
		}
	},
}

func filterSessions(sessions []*runner.SessionState, status string) []*runner.SessionState {
	if status == "" {
		return sessions
	}
	var filtered []*runner.SessionState
	for _, s := range sessions {
		if strings.EqualFold(s.Status, status) {
			filtered = append(filtered, s)
		}
	}
	return filtered
}

func sortSessions(sessions []*runner.SessionState, sortBy string) {
	switch sortBy {
	case "name":
		sort.SliceStable(sessions, func(i, j int) bool {
			return sessions[i].Name < sessions[j].Name
		})
	case "start_time":
		sort.SliceStable(sessions, func(i, j int) bool {
			return sessions[i].StartTime.Before(sessions[j].StartTime)
		})
	}
}

func printSessions(writer io.Writer, sessions []*runner.SessionState) {
	w := tabwriter.NewWriter(writer, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "NAME\tSTATUS\tPID\tSTARTED\tWORKSPACE")
	fmt.Fprintln(w, "----\t------\t---\t-------\t---------")

	for _, session := range sessions {
		started := session.StartTime.Format(time.RFC3339)
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%s\n",
			session.Name,
			session.Status,
			session.PID,
			started,
			session.Workspace,
		)
	}
	w.Flush()
}

func printSessionsJSON(writer io.Writer, sessions []*runner.SessionState) {
	jsonBytes, err := json.MarshalIndent(sessions, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to marshal sessions to JSON: %v\n", err)
		exit(1)
	}
	fmt.Fprintln(writer, string(jsonBytes))
}
