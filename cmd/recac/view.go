package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"io"
	"sort"
)

var showSensitive bool

var viewCmd = &cobra.Command{
	Use:   "view",
	Short: "View the current configuration",
	Long:  `View the current configuration. Sensitive values are redacted by default.`,
	Run: func(cmd *cobra.Command, args []string) {
		settings := viper.AllSettings()
		printConfig(cmd.OutOrStdout(), settings, "", showSensitive)
	},
}

func init() {
	viewCmd.Flags().BoolVar(&showSensitive, "show-sensitive", false, "Show sensitive values")
	configCmd.AddCommand(viewCmd)
}

func printConfig(writer io.Writer, settings map[string]interface{}, prefix string, showSensitive bool) {
	keys := make([]string, 0, len(settings))
	for key := range settings {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := settings[key]
		fullKey := prefix + key
		if isSensitive(fullKey) && !showSensitive {
			value = "[REDACTED]"
		}

		switch v := value.(type) {
		case map[string]interface{}:
			printConfig(writer, v, fullKey+".", showSensitive)
		default:
			fmt.Fprintf(writer, "%s: %v\n", fullKey, value)
		}
	}
}

