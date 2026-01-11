package main

import (
	"fmt"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var inspectCmd = &cobra.Command{
	Use:   "inspect [SESSION_NAME]",
	Short: "Display a comprehensive summary of a session",
	Long:  `Inspect provides a detailed view of a specific session, including metadata, token usage, costs, errors, git changes, and recent logs.`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		session, err := sm.LoadSession(sessionName)
		if err != nil {
			return err
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
