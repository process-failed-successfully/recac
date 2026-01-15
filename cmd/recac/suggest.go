package main

import (
	"fmt"
	"sort"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(suggestCmd)
}

var suggestCmd = &cobra.Command{
	Use:   "suggest",
	Short: "Suggest the next logical command to run",
	Long:  `Analyzes the most recent session to suggest a helpful next command.`,
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
			cmd.Println("No sessions found. Why not start with `recac start`?")
			return nil
		}

		sort.Slice(sessions, func(i, j int) bool {
			return sessions[i].StartTime.After(sessions[j].StartTime)
		})

		latestSession := sessions[0]

		switch latestSession.Status {
		case "running":
			cmd.Printf("Session '%s' is running. You could:\n", latestSession.Name)
			cmd.Printf("- Follow its logs: recac logs -f %s\n", latestSession.Name)
			cmd.Printf("- See its status: recac status %s\n", latestSession.Name)
		case "completed":
			cmd.Printf("Session '%s' completed successfully. You could:\n", latestSession.Name)
			cmd.Printf("- View its history: recac history %s\n", latestSession.Name)
			cmd.Printf("- Archive it: recac archive %s\n", latestSession.Name)
			cmd.Printf("- Start a new session: recac start\n")
		case "error":
			cmd.Printf("Session '%s' failed. You could:\n", latestSession.Name)
			cmd.Printf("- Check the logs: recac logs %s\n", latestSession.Name)
			cmd.Printf("- View its history: recac history %s\n", latestSession.Name)
		default:
			cmd.Printf("Latest session '%s' has status '%s'.\n", latestSession.Name, latestSession.Status)
		}

		return nil
	},
}
