package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var replayCmd = &cobra.Command{
	Use:   "replay <session_name>",
	Short: "Replay a previous session from its starting state",
	Long: `Replay a previous session by creating a new session with the same
initial configuration and git commit SHA. This is useful for debugging
or reproducing results.

The workspace will be reset to the original session's starting commit.
A new session will be created with a unique name (e.g., <original_name>-replay-TIMESTAMP)
and started in the background.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Replaying session '%s'...\n", sessionName)
		newSession, err := sm.ReplaySession(sessionName)
		if err != nil {
			return fmt.Errorf("failed to replay session: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully started replayed session '%s' (PID: %d).\n", newSession.Name, newSession.PID)
		fmt.Fprintf(cmd.OutOrStdout(), "Logs are available at: %s\n", newSession.LogFile)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(replayCmd)
}
