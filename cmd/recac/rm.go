package main

import (
	"errors"
	"fmt"
	"strings"

	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var (
	forceRemove bool
)

var rmCmd = &cobra.Command{
	Use:   "rm [SESSION_NAME]...",
	Short: "Remove one or more sessions",
	Long:  `Remove one or more sessions. This will delete the session's state and log files.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}
		return runRmCmd(sm, cmd, args)
	},
}

func runRmCmd(sm ISessionManager, cmd *cobra.Command, args []string) error {
	var errorMessages []string
	for _, sessionName := range args {
		err := sm.RemoveSession(sessionName, forceRemove)
		if err != nil {
			// Check for the specific error to provide a clean user message.
			if errors.Is(err, runner.ErrSessionRunning) {
				// This is a specific condition, not a failure.
				fmt.Fprintf(cmd.ErrOrStderr(), "Skipping running session '%s'. Use --force to remove.\n", sessionName)
			} else {
				errorMessages = append(errorMessages, fmt.Sprintf("Failed to remove session '%s': %s", sessionName, err.Error()))
			}
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Removed session '%s'\n", sessionName)
	}

	if len(errorMessages) > 0 {
		return fmt.Errorf("encountered errors:\n- %s", strings.Join(errorMessages, "\n- "))
	}

	return nil
}

func init() {
	rmCmd.Flags().BoolVarP(&forceRemove, "force", "f", false, "Force remove a running session")
	rootCmd.AddCommand(rmCmd)
}
