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

var listKeysCmd = &cobra.Command{
	Use:   "list-keys",
	Short: "List all configuration keys",
	RunE:  listKeys,
}

var listModelsCmd = &cobra.Command{
	Use:   "list-models",
	Short: "List available models",
	RunE:  listModels,
}

func init() {
	configCmd.AddCommand(listKeysCmd)
	configCmd.AddCommand(listModelsCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSearchCmd)
	configCmd.AddCommand(configDiffCmd)
	configCmd.AddCommand(setCmd)
	configCmd.AddCommand(unsetCmd)
	configCmd.AddCommand(configExportCmd)
	// configResetCmd is added in cmd/recac/config_reset.go init()
	rootCmd.AddCommand(configCmd)
}
