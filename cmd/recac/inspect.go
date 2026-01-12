package main

import (
	"fmt"
	"recac/internal/runner"
	"sort"

	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect [SESSION_NAME]",
	Short: "Display a comprehensive summary of a session",
	Long:  `Inspect provides a detailed view of a specific session, including metadata, token usage, costs, errors, git changes, and recent logs. If no session name is provided, it inspects the most recent session.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var session *runner.SessionState
		if len(args) == 1 {
			// Existing behavior: inspect a specific session
			session, err = sm.LoadSession(args[0])
			if err != nil {
				return err
			}
		} else {
			// New behavior: inspect the most recent session
			sessions, err := sm.ListSessions()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}
			if len(sessions) == 0 {
				cmd.Println("No sessions found.")
				return nil
			}

			// Sort sessions by start time to find the most recent
			sort.Slice(sessions, func(i, j int) bool {
				return sessions[i].StartTime.After(sessions[j].StartTime)
			})
			session = sessions[0]
		}

		return displaySessionDetail(cmd, session)
	},
}

func init() {
	rootCmd.AddCommand(inspectCmd)
}

func displaySessionDetail(cmd *cobra.Command, session *runner.SessionState) error {
	return DisplaySessionDetail(cmd, session, false)
}
