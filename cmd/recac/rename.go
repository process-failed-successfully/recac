package main

import (
	"errors"
	"fmt"

	"recac/internal/runner"

	"github.com/spf13/cobra"
)

var renameCmd = &cobra.Command{
	Use:          "rename [OLD_NAME] [NEW_NAME]",
	Short:        "Rename a session",
	Long:         `Rename a session. This will rename the session's state and log files.`,
	Args:         cobra.ExactArgs(2),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}
		return runRenameCmd(sm, cmd, args)
	},
}

func runRenameCmd(sm ISessionManager, cmd *cobra.Command, args []string) error {
	oldName := args[0]
	newName := args[1]

	err := sm.RenameSession(oldName, newName)
	if err != nil {
		if errors.Is(err, runner.ErrSessionRunning) {
			return fmt.Errorf("cannot rename running session '%s'. Please stop it first", oldName)
		}
		return fmt.Errorf("failed to rename session '%s': %w", oldName, err)
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Renamed session '%s' to '%s'\n", oldName, newName)
	return nil
}

func init() {
	rootCmd.AddCommand(renameCmd)
}
