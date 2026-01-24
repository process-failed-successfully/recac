package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(recoverCmd)
}

var recoverCmd = &cobra.Command{
	Use:   "recover [session-name]",
	Short: "Recover a dead session",
	Long: `Restarts a session that has crashed or died unexpectedly.
It reuses the original configuration (command, workspace, logs) and appends to the log file.
Unlike 'resume', which is for paused sessions, 'recover' handles sessions where the process is gone.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		session, err := sm.RecoverSession(sessionName)
		if err != nil {
			return err
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Session '%s' recovered successfully!\n", session.Name)
		fmt.Fprintf(cmd.OutOrStdout(), "New PID: %d\n", session.PID)
		fmt.Fprintf(cmd.OutOrStdout(), "Status: %s\n", session.Status)
		fmt.Fprintf(cmd.OutOrStdout(), "Logs: %s\n", session.LogFile)

		return nil
	},
}
