package main

import (
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Manage configuration for the recac CLI.`,
}

func init() {
	rootCmd.AddCommand(configCmd)
}
