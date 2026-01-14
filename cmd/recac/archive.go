package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var archiveCmd = &cobra.Command{
	Use:   "archive [SESSION_NAME]...",
	Short: "Archive one or more sessions",
	Long:  `Archive one or more sessions. This moves the session's state and log files to a separate directory, hiding it from the main list.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var errorMessages []string
		for _, sessionName := range args {
			err := sm.ArchiveSession(sessionName)
			if err != nil {
				errorMessages = append(errorMessages, fmt.Sprintf("Failed to archive session '%s': %s", sessionName, err.Error()))
				continue
			}
			fmt.Fprintf(cmd.OutOrStdout(), "Archived session '%s'\n", sessionName)
		}

		if len(errorMessages) > 0 {
			return fmt.Errorf("encountered errors:\n- %s", strings.Join(errorMessages, "\n- "))
		}

		return nil
	},
}

func init() {
	rootCmd.AddCommand(archiveCmd)
}
