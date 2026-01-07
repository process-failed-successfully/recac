package main

import (
	"fmt"
	"os"
	"recac/internal/runner"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(listCmd)
}

var listCmd = &cobra.Command{
	Use:        "list",
	Short:      "List all sessions (DEPRECATED, use 'status')",
	Long:       `List all active and completed sessions. This command is deprecated; please use 'recac status' instead.`,
	Deprecated: "use 'recac status' instead.",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(os.Stderr, "Warning: 'recac list' is deprecated and will be removed in a future version. Please use 'recac status'.")
		sm, err := runner.NewSessionManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to create session manager: %v\n", err)
			exit(1)
		}
		if err := showStatus(sm); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}
	},
}
