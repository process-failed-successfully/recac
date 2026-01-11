package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var fixFlag bool

// checkCmd represents the check command
// checkCmd is deprecated and now forwards to the 'doctor' command.
var checkCmd = &cobra.Command{
	Use:        "check",
	Short:      "DEPRECATED: Use 'doctor' instead. Check dependencies and environment",
	Long:       `DEPRECATED: This command is now an alias for 'doctor'. Please use 'recac doctor' for environment checks.`,
	Deprecated: "use 'recac doctor' instead.",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("⚠️  Warning: The 'check' command is deprecated. Please use 'doctor' instead.")
		fmt.Println()
		// Forward the call to the doctor command
		return doctorCmd.RunE(cmd, args)
	},
}

func init() {
	// We keep the flags for backward compatibility
	checkCmd.Flags().BoolVar(&fixFlag, "fix", false, "Attempt to fix issues automatically")
	// Intentionally not adding to rootCmd, as doctorCmd has an alias.
	// We keep the file to avoid breaking old references and to hold the deprecation notice.
	// rootCmd.AddCommand(checkCmd)
}
