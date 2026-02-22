package main

import (
	"fmt"
	"recac/internal/cmdutils"

	"github.com/spf13/cobra"
)

var fixFlag bool

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check dependencies and environment (Deprecated)",
	Long: `Deprecated: Use 'recac doctor' instead.
Perform pre-flight checks on the environment and dependencies.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), "⚠️  Deprecated: 'recac check' is deprecated. Please use 'recac doctor' for comprehensive diagnostics.")
		fmt.Fprintln(cmd.OutOrStdout(), "")

		// Run doctor logic
		fmt.Fprint(cmd.OutOrStdout(), cmdutils.GetDoctor())
	},
}

func init() {
	checkCmd.Flags().BoolVar(&fixFlag, "fix", false, "Attempt to fix issues automatically (Deprecated/Ignored)")
	rootCmd.AddCommand(checkCmd)
}
