package main

import (
	"fmt"
	"recac/internal/git"

	"github.com/spf13/cobra"
)

func init() {
	featureCmd.AddCommand(featureStartCmd)
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

		err := git.NewClient().CheckoutNewBranch("", branchName)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			exit(1)
		}

		fmt.Printf("Successfully switched to branch %s\n", branchName)
	},
}
