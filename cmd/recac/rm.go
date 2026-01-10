package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(rmCmd)
	rmCmd.Flags().BoolP("force", "f", false, "Force the removal of a running session")
}

var rmCmd = &cobra.Command{
	Use:   "rm [SESSION_NAME]...",
	Short: "Remove one or more sessions",
	Long:  `Remove one or more sessions by name. This will delete the session's state file and log file from disk.`,
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		force, _ := cmd.Flags().GetBool("force")
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		var errs []error
		for _, name := range args {
			session, err := sm.LoadSession(name)
			if err != nil {
				errs = append(errs, fmt.Errorf("session '%s' not found", name))
				continue
			}

			if session.Status == "running" {
				if !force {
					errs = append(errs, fmt.Errorf("cannot remove running session '%s' without --force", name))
					continue
				}
				if err := sm.StopSession(name); err != nil {
					errs = append(errs, fmt.Errorf("failed to stop session '%s': %w", name, err))
					continue
				}
			}

			if err := sm.RemoveSession(name); err != nil {
				errs = append(errs, fmt.Errorf("failed to remove session '%s': %w", name, err))
				continue
			}
			cmd.Printf("Removed session: %s\n", name)
		}

		if len(errs) > 0 {
			// Print all errors and return a final error
			for _, e := range errs {
				cmd.PrintErrln(e)
			}
			return fmt.Errorf("failed to remove one or more sessions")
		}

		return nil
	},
}
