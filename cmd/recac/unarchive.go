package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var unarchiveAll bool

var unarchiveCmd = &cobra.Command{
	Use:   "unarchive [SESSION_NAME]...",
	Short: "Unarchive one or more sessions",
	Long:  `Unarchive one or more sessions. This moves the session's state and log files back to the active directory.`,
	Args:  cobra.ArbitraryArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		if !unarchiveAll && len(args) == 0 {
			return fmt.Errorf("you must specify at least one session to unarchive, or use the --all flag")
		}
		if unarchiveAll && len(args) > 0 {
			return fmt.Errorf("you cannot specify session names when using the --all flag")
		}

		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var sessionsToUnarchive []string
		if unarchiveAll {
			archivedSessions, err := sm.ListArchivedSessions()
			if err != nil {
				return fmt.Errorf("failed to list archived sessions: %w", err)
			}
			for _, s := range archivedSessions {
				sessionsToUnarchive = append(sessionsToUnarchive, s.Name)
			}
		} else {
			sessionsToUnarchive = args
		}

		if len(sessionsToUnarchive) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No sessions to unarchive.")
			return nil
		}

		var errorMessages []string
		for _, sessionName := range sessionsToUnarchive {
			err := sm.UnarchiveSession(sessionName)
			if err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("Failed to unarchive session '%s': %s", sessionName, err.Error()))
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Unarchived session '%s'\n", sessionName)
		}

		if len(errorMessages) > 0 {
			return fmt.Errorf("encountered errors:\n- %s", strings.Join(errorMessages, "\n- "))
		}

		return nil
	},
}

func init() {
	unarchiveCmd.Flags().BoolVar(&unarchiveAll, "all", false, "Unarchive all sessions")
	rootCmd.AddCommand(unarchiveCmd)
}
