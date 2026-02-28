package main

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var setCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a configuration value",
	Long: `Set a configuration value for the recac CLI.

This updates the configuration file with the new value. If the key is already set,
it will be overwritten. Types are automatically parsed (e.g. 'true' to boolean, '42' to integer).

Examples:
  recac config set agent.provider openai
  recac config set max_iterations 10
  recac config set notifications.slack.enabled true`,
	Args: cobra.ExactArgs(2),
	RunE: setConfigKey,
}

func setConfigKey(cmd *cobra.Command, args []string) error {
	key := args[0]
	rawValue := args[1]

	// Parse value type
	var value interface{}
	if b, err := strconv.ParseBool(rawValue); err == nil {
		value = b
	} else if i, err := strconv.ParseInt(rawValue, 10, 64); err == nil {
		value = int(i)
	} else if f, err := strconv.ParseFloat(rawValue, 64); err == nil {
		value = f
	} else {
		value = rawValue
	}

	viper.Set(key, value)

	cfgFile := viperConfigFileUsed()
	if cfgFile == "" {
		cfgFile = "config.yaml"
	}

	// Try writing to the existing config file
	err := viperSafeWriteConfig()
	if err != nil {
		err = viper.WriteConfigAs(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to write config to %s: %w", cfgFile, err)
		}
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Successfully set %s to %v\n", key, rawValue)
	return nil
}
