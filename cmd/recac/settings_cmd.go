package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(settingsCmd)
	settingsCmd.AddCommand(settingsViewCmd)
	settingsCmd.AddCommand(settingsGetCmd)
	settingsCmd.AddCommand(settingsSetCmd)
}

var settingsCmd = &cobra.Command{
	Use:   "settings",
	Short: "Manage configuration settings",
	Long:  `Manage the application's configuration settings stored in config.yaml.`,
}

var settingsViewCmd = &cobra.Command{
	Use:   "view",
	Short: "View all configuration settings",
	RunE: func(cmd *cobra.Command, args []string) error {
		allSettings := viper.AllSettings()
		if len(allSettings) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No configuration settings found.")
			return nil
		}

		// A simple way to pretty-print the config.
		// In a real-world scenario, you might use a YAML marshaller.
		for key, value := range allSettings {
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %v\n", key, value)
		}
		return nil
	},
}

var settingsGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		if !viper.IsSet(key) {
			return fmt.Errorf("key not found: %s", key)
		}
		value := viper.Get(key)
		fmt.Fprintln(cmd.OutOrStdout(), value)
		return nil
	},
}

var settingsSetCmd = &cobra.Command{
	Use:   "set [key] [value]",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := args[1]

		// For nested keys, we might need more sophisticated logic
		// But for now, we'll handle simple top-level keys.
		viper.Set(key, value)

		// Get the path of the config file viper is using.
		configFile := viper.ConfigFileUsed()

		if configFile != "" {
			// If a config file is being used, write the changes back to that same file.
			if err := viper.WriteConfigAs(configFile); err != nil {
				return fmt.Errorf("error writing to config file %s: %w", configFile, err)
			}
		} else {
			// If no config file is in use, attempt to create a new one.
			// This relies on the search paths configured in root.go (e.g., creating 'config.yaml').
			if err := viper.WriteConfig(); err != nil {
				return fmt.Errorf("error creating new config file: %w", err)
			}
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Set %s = %s\n", key, value)
		return nil
	},
}
