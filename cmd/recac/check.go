package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var fixFlag bool

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check dependencies and environment (Deprecated)",
	Long: `Perform pre-flight checks on the environment and dependencies.
DEPRECATED: Use 'recac doctor' instead.
Use --fix to automatically attempt repairs for minor issues (configuration).`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("⚠️  WARNING: 'recac check' is deprecated. Please use 'recac doctor' instead.")

		if fixFlag {
			fmt.Println("Attempting to fix configuration...")
			if err := fixConfig(); err != nil {
				fmt.Printf("  Failed to fix config: %v\n", err)
			} else {
				fmt.Printf("  ✅ Config fixed (created default)\n")
			}
		}

		fmt.Println("Running 'doctor' checks...")
		// Delegate to doctor command
		doctorCmd.Run(cmd, args)
	},
}

func init() {
	checkCmd.Flags().BoolVar(&fixFlag, "fix", false, "Attempt to fix issues automatically")
	rootCmd.AddCommand(checkCmd)
}

func fixConfig() error {
	// Simple fix: create default config if missing
	viper.SetDefault("provider", "gemini")
	viper.SetDefault("model", "gemini-pro")
	return viper.SafeWriteConfig()
}
