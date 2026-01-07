package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	configCmd.AddCommand(configViewCmd)
	configCmd.AddCommand(configGetCmd)
	configCmd.AddCommand(configSetCmd)
	configCmd.AddCommand(configListModelsCmd)
	rootCmd.AddCommand(configCmd)
}

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration settings",
	Long:  `View and manage the configuration for recac.`,
}

var configViewCmd = &cobra.Command{
	Use:   "view",
	Short: "Display all configuration settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		allSettings := viper.AllSettings()
		if len(allSettings) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No configuration settings found.")
			return nil
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Current configuration:")
		for key, value := range allSettings {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %v\n", key, value)
		}
		return nil
	},
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a specific configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		if !viper.IsSet(key) {
			return fmt.Errorf("key not found in configuration: %s", key)
		}
		value := viper.Get(key)
		fmt.Fprintf(cmd.OutOrStdout(), "%v\n", value)
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		viper.Set(key, value)
		if err := viper.SafeWriteConfig(); err != nil {
			// If file doesn't exist, we might need to create it.
			// Let's try to get the config file path and write it.
			cfgFile := viper.ConfigFileUsed()
			if cfgFile == "" {
				// Try to get from flags if available, or use default.
				if rootCmd.PersistentFlags().Changed("config") {
					cfgFile, _ = rootCmd.PersistentFlags().GetString("config")
				} else {
					home, _ := os.UserHomeDir()
					cfgFile = home + "/.recac.yaml"
				}
			}
			if err := viper.WriteConfigAs(cfgFile); err != nil {
				return fmt.Errorf("error writing config file: %w", err)
			}
		}
		fmt.Fprintf(cmd.OutOrStdout(), "Updated configuration key '%s'\n", key)
		return nil
	},
}

var configListModelsCmd = &cobra.Command{
	Use:   "list-models",
	Short: "List available AI models for the configured provider",
	RunE: func(cmd *cobra.Command, args []string) error {
		provider := viper.GetString("agent_provider")
		if provider == "" {
			provider = "gemini" // Default to gemini if not set
		}
		modelFileName := fmt.Sprintf("%s-models.json", provider)

		// This assumes the model files are in the same directory as the executable or cwd.
		// This might need adjustment based on how the application is deployed.
		data, err := os.ReadFile(modelFileName)
		if err != nil {
			return fmt.Errorf("could not read models file for provider '%s': %w. Ensure '%s' is present", provider, err, modelFileName)
		}

		var modelsData struct {
			Models []struct {
				Name        string `json:"name"`
				DisplayName string `json:"displayName"`
			} `json:"models"`
		}

		if err := json.Unmarshal(data, &modelsData); err != nil {
			return fmt.Errorf("failed to parse models file: %w", err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Available models for %s:\n", provider)
		for _, model := range modelsData.Models {
			fmt.Fprintf(cmd.OutOrStdout(), "- %s (%s)\n", model.DisplayName, model.Name)
		}

		return nil
	},
}
