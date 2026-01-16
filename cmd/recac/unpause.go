package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(unpauseCmd)
}

var unpauseCmd = &cobra.Command{
	Use:   "unpause [session-name]",
	Short: "Unpause a paused session",
	Long:  `Unpause a paused session by sending it the SIGCONT signal.`,
	Args:  cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var sessionName string
		var err error

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		if len(args) == 0 {
			sessionName, err = interactiveSessionSelect(sm, "paused", "Choose a session to unpause:")
			if err != nil {
				return err
			}
		} else {
			sessionName = args[0]
		}

		if err := sm.ResumeSession(sessionName); err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Session '%s' unpaused successfully\n", sessionName)
		return nil
	},
}
