package main

import (
	"encoding/json"
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

// listModelsCmd represents the list-models command
var listModelsCmd = &cobra.Command{
	Use:   "list-models",
	Short: "List compatible models for the active provider",
	Run: func(cmd *cobra.Command, args []string) {
		provider := viper.GetString("provider")
		fmt.Printf("Listing models for provider: %s\n", provider)

		var filename string
		switch provider {
		case "gemini":
			filename = "gemini-models.json"
		case "openrouter":
			filename = "openrouter-models.json"
		default:
			fmt.Printf("Model listing not supported for provider: %s\n", provider)
			return
		}

		// Read file
		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Printf("Error reading models file %s: %v\n", filename, err)
			return
		}

		// Parse
		var modelList struct {
			Models []struct {
				Name        string `json:"name"`
					DisplayName string `json:"displayName"`
			} `json:"models"`
		}

		if err := json.Unmarshal(data, &modelList); err != nil {
			fmt.Printf("Error parsing models file: %v\n", err)
			return
		}

		for _, m := range modelList.Models {
			if m.DisplayName != "" {
				fmt.Printf("- %s (%s)\n", m.Name, m.DisplayName)
			} else {
				fmt.Printf("- %s\n", m.Name)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(setCmd)
	configCmd.AddCommand(listKeysCmd)
	configCmd.AddCommand(listModelsCmd)
}