package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(NewStopCmd())
}

func NewStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop [session-name]",
		Short: "Stop a running session",
		Long:  `Stop a running session gracefully. Sends SIGTERM first, then SIGKILL if needed.`,
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var sessionName string
			var err error

			sm, err := sessionManagerFactory()
			if err != nil {
				return fmt.Errorf("failed to create session manager: %w", err)
			}

			if len(args) == 0 {
				sessionName, err = interactiveSessionSelect(sm, "running", "Choose a session to stop:")
				if err != nil {
					return err
				}
			} else {
				sessionName = args[0]
			}

			if err := sm.StopSession(sessionName); err != nil {
				return err
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Session '%s' stopped successfully\n", sessionName)
			return nil
		},
	}
}
