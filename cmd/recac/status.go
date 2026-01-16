package main

import (
	"errors"
	"fmt"
	"os"
	"sort"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status [SESSION_NAME]",
	Short: "Display a snapshot of a session's agent state",
	Long:  `Provides a real-time summary of a running or completed session, including token usage, cost, and the agent's last known activity.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var sessionName string
		if len(args) > 0 {
			sessionName = args[0]
		} else {
			// If no name is provided, find the most recent session
			sessions, err := sm.ListSessions()
			if err != nil {
				return fmt.Errorf("failed to list sessions: %w", err)
			}
			if len(sessions) == 0 {
				cmd.Println("No sessions found.")
				return nil
			}
			sort.Slice(sessions, func(i, j int) bool {
				return sessions[i].StartTime.After(sessions[j].StartTime)
			})
			sessionName = sessions[0].Name
			cmd.Printf("No session name provided, showing status for most recent session: %s\n\n", sessionName)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("could not load session '%s': %w", sessionName, err)
		}

		// Load the agent's state
		agentState, err := loadAgentState(session.AgentStateFile)
		if err != nil {
			// If the state file doesn't exist, print basic session info and exit gracefully
			if errors.Is(err, os.ErrNotExist) {
				cmd.Printf("Session '%s' found, but agent state is not available.\n", session.Name)
				cmd.Printf("Status: %s\n", session.Status)
				return nil
			}
			return fmt.Errorf("could not load agent state for session '%s': %w", sessionName, err)
		}

		// --- Display Status ---
		displayStatus(cmd, session, agentState)

		return nil
	},
}
