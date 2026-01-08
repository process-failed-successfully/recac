package main

import (
	"errors"
	"fmt"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var deleteAll bool
var force bool

// newDeleteSessionManager allows for mocking the session manager in tests.
var newDeleteSessionManager = runner.NewSessionManager

func init() {
	deleteCmd := &cobra.Command{
		Use:   "delete [session-name]",
		Short: "Delete a session or all sessions",
		Long:  `Delete a specific session by name, or all sessions using the --all flag.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  runDeleteCmd,
	}
	deleteCmd.Flags().BoolVar(&deleteAll, "all", false, "Delete all sessions")
	deleteCmd.Flags().BoolVarP(&force, "force", "f", false, "Skip confirmation prompt")
	rootCmd.AddCommand(deleteCmd)
}

func runDeleteCmd(cmd *cobra.Command, args []string) error {
	if !deleteAll && len(args) == 0 {
		return fmt.Errorf("you must specify a session name or use the --all flag")
	}
	if deleteAll && len(args) > 0 {
		return fmt.Errorf("you cannot specify a session name when using the --all flag")
	}

	sm, err := newDeleteSessionManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	if deleteAll {
		if !force {
			fmt.Fprint(cmd.OutOrStdout(), "Are you sure you want to delete all sessions? [y/N]: ")
			var response string
			fmt.Fscanln(cmd.InOrStdin(), &response)
			if response != "y" && response != "Y" {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
		}

		sessions, err := sm.ListSessions()
		if err != nil {
			return fmt.Errorf("failed to list sessions: %w", err)
		}

		if len(sessions) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No sessions found to delete.")
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Deleting all sessions...")
		for _, session := range sessions {
			if err := sm.DeleteSession(session.Name); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error deleting session '%s': %v\n", session.Name, err)
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "Deleted session: %s\n", session.Name)
			}
		}
		fmt.Fprintln(cmd.OutOrStdout(), "All sessions deleted.")

	} else {
		sessionName := args[0]
		if !force {
			fmt.Fprintf(cmd.OutOrStdout(), "Are you sure you want to delete session '%s'? [y/N]: ", sessionName)
			var response string
			fmt.Fscanln(cmd.InOrStdin(), &response)
			if response != "y" && response != "Y" {
				fmt.Fprintln(cmd.OutOrStdout(), "Aborted.")
				return nil
			}
		}

		if err := sm.DeleteSession(sessionName); err != nil {
			if errors.Is(err, runner.ErrSessionNotFound) {
				return fmt.Errorf("session '%s' not found", sessionName)
			}
			return fmt.Errorf("failed to delete session: %w", err)
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Session '%s' deleted successfully.\n", sessionName)
	}

	return nil
}
