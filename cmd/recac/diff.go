package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var diffCmd = &cobra.Command{
	Use:   "diff [session_a] [session_b]",
	Short: "Compare two sessions",
	Long:  "Compares two sessions.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionAName := args[0]
		sessionBName := args[1]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		sessionA, err := sm.LoadSession(sessionAName)
		if err != nil {
			return fmt.Errorf("failed to load session %s: %w", sessionAName, err)
		}

		sessionB, err := sm.LoadSession(sessionBName)
		if err != nil {
			return fmt.Errorf("failed to load session %s: %w", sessionBName, err)
		}

		return DisplaySessionDiff(cmd, sessionA, sessionB)
	},
}

func init() {
	rootCmd.AddCommand(diffCmd)
}
