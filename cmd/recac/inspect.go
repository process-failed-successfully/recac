package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(inspectCmd)
}

var inspectCmd = &cobra.Command{
	Use:   "inspect <session-name>",
	Short: "Display a comprehensive summary of a single session",
	Long: `Provides a detailed view of a specific session, combining metadata,
token usage, cost, and recent logs into a single summary.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		if sessionName == "" {
			return errors.New("session name cannot be empty")
		}

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
		}

		return DisplaySessionDetail(cmd, session, false)
	},
}
