package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage configuration",
	Long:  `Get, set, and list configuration values.`, 
}

// setCmd represents the set command
var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		valueStr := args[1]
		var value interface{}

		// Try to parse as integer
		if i, err := strconv.Atoi(valueStr); err == nil {
			value = i
		} else if b, err := strconv.ParseBool(valueStr); err == nil {
			// Try to parse as boolean
			value = b
		} else {
			// Fallback to string
			value = valueStr
		}

		viper.Set(key, value)

		// Save configuration
		filename := viper.ConfigFileUsed()
		if filename == "" {
			// If no config file was used (e.g. defaults only), default to config.yaml in current dir
			filename = "config.yaml"
		}

		if err := viper.WriteConfigAs(filename); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing config to %s: %v\n", filename, err)
			os.Exit(1)
		}

		fmt.Printf("Configuration updated: %s = %v\n", key, value)
		fmt.Printf("Saved to: %s\n", filename)
	},
}

// listKeysCmd represents the list-keys command
var listKeysCmd = &cobra.Command{
	Use:   "list-keys",
	Short: "List all available configuration keys",
	Run: func(cmd *cobra.Command, args []string) {
		keys := viper.AllKeys()
		fmt.Println("Available configuration keys:")
		for _, key := range keys {
			fmt.Printf("- %s: %v\n", key, viper.Get(key))
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(setCmd)
	configCmd.AddCommand(listKeysCmd)
}
