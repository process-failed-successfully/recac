package main

import (
	"fmt"
	"github.com/spf13/cobra"
)

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:     "check",
	Short:   "DEPRECATED: Use the 'doctor' command instead.",
	Long:    `DEPRECATED: This command is now an alias for 'doctor'.`,
	Aliases: []string{"doctor-alias"}, // Keep it hidden but functional
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("⚠️  Warning: The 'check' command is deprecated and will be removed in a future version.")
		fmt.Println("Please use the 'doctor' command instead.")
		fmt.Println()

		// Forward to the doctor command
		return doctorCmd.RunE(cmd, args)
	},
}

func init() {
	// The --fix flag is ignored but kept for backward compatibility to avoid errors
	checkCmd.Flags().Bool("fix", false, "Attempt to fix issues automatically (ignored)")
	rootCmd.AddCommand(checkCmd)
}
