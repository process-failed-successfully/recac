package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(stopCmd)
}

var stopCmd = &cobra.Command{
	Use:   "stop [session-name]",
	Short: "Stop a running session",
	Long:  `Stop a running session gracefully. Sends SIGTERM first, then SIGKILL if needed.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		sessionName := args[0]

		sm, err := newSessionManager()
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: failed to create session manager: %v\n", err)
			exit(1)
		}

		if err := sm.StopSession(sessionName); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			exit(1)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Session '%s' stopped successfully\n", sessionName)
	},
}
