package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(pauseCmd)
}

var pauseCmd = &cobra.Command{
	Use:   "pause [session-name]",
	Short: "Pause a running session",
	Long:  `Pause a running session by sending it the SIGSTOP signal.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionName string
		var err error

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		if len(args) == 0 {
			sessionName, err = interactiveSessionSelect(sm, "running", "Choose a session to pause:")
			if err != nil {
				return err
			}
		} else {
			sessionName = args[0]
		}

		if err := sm.PauseSession(sessionName); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Session '%s' paused successfully\n", sessionName)
		return nil
	},
}
