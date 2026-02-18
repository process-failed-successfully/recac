package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

// initHistoryCmd initializes the history command and adds it to the root command.
func initHistoryCmd(rootCmd *cobra.Command) {
	var fullLogs bool
	historyCmd := &cobra.Command{
		Use:   "history [session-name]",
		Short: "Show history of a specific session",
		Long:  `Displays detailed history for a specific RECAC session.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			sm, err := sessionManagerFactory()
			if err != nil {
				return fmt.Errorf("failed to initialize session manager: %w", err)
			}
			sessionName := args[0]
			session, err := sm.LoadSession(sessionName)
			if err != nil {
				return fmt.Errorf("failed to load session '%s': %w", sessionName, err)
			}
			return DisplaySessionDetail(cmd, session, fullLogs)
		},
	}
	historyCmd.Flags().BoolVar(&fullLogs, "full-logs", false, "Display full log file content")
	rootCmd.AddCommand(historyCmd)
}
