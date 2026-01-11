package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(showCmd)
}

var showCmd = &cobra.Command{
	Use:   "show [session-name]",
	Short: "Show a git diff of the work done in a session",
	Long: `Displays the git diff between the starting and ending commits of a completed session.
This command helps you review the exact changes made by the agent during its run.`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to initialize session manager: %w", err)
		}

		return handleSingleSessionDiff(cmd, sm, args[0])
	},
}
