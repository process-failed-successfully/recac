package main

import (
	"fmt"
	"os"
	"recac/internal/cmdutils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var fixFlag bool

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check dependencies and environment (Deprecated)",
	Long: `Perform pre-flight checks on the environment and dependencies.
Use --fix to automatically attempt repairs for minor issues.

DEPRECATED: This command is deprecated and will be removed in a future release. Please use 'doctor' instead.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  'check' is deprecated and will be removed in a future release. Please use 'doctor' instead.")

		if fixFlag {
			if viper.ConfigFileUsed() == "" {
				if err := cmdutils.FixConfig(); err != nil {
					fmt.Printf("Failed to fix config: %v\n", err)
				} else {
					fmt.Println("Config fixed (created default)")
				}
			}
		}

		report, passed := cmdutils.GetDoctor()
		fmt.Fprint(cmd.OutOrStdout(), report)

		if !passed {
			os.Exit(1)
		}
	},
}

func init() {
	checkCmd.Flags().BoolVar(&fixFlag, "fix", false, "Attempt to fix issues automatically")
	rootCmd.AddCommand(checkCmd)
}
