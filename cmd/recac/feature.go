package main

import (
	"fmt"
	"recac/internal/pkg/git"

	"github.com/spf13/cobra"
)

func init() {
	featureCmd.AddCommand(featureStartCmd)
	featureCmd.AddCommand(featureAbortCmd)
	rootCmd.AddCommand(featureCmd)
}

var featureCmd = &cobra.Command{
	Use:   "feature",
	Short: "Manage features",
}

var featureStartCmd = &cobra.Command{
	Use:   "start [name]",
	Short: "Start a new feature",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		branchName := fmt.Sprintf("feature/%s", name)

		fmt.Printf("Starting feature: %s\n", name)
		fmt.Printf("Creating branch: %s\n", branchName)

		err := git.CreateBranch(branchName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			exit(1)
		}

		fmt.Printf("Successfully switched to branch %s\n", branchName)
	},
}

var featureAbortCmd = &cobra.Command{
	Use:   "abort [name]",
	Short: "Abort a feature and delete its branch",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		name := args[0]
		branchName := fmt.Sprintf("feature/%s", name)

		fmt.Printf("Aborting feature: %s\n", name)
		fmt.Printf("Deleting branch: %s\n", branchName)

		err := git.AbortFeature(branchName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			exit(1)
		}

		fmt.Printf("Successfully deleted branch %s and switched to main\n", branchName)
	},
}
