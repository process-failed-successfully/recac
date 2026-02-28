package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configGetCmd = &cobra.Command{
	Use:   "get <key>",
	Short: "Get a configuration value",
	Long: `Get a configuration value for the recac CLI.

This retrieves the configuration value for the given key from the configuration file or environment variables.
Sensitive keys are redacted by default.

Examples:
  recac config get agent.provider
  recac config get max_iterations
  recac config get --show-sensitive secrets.openai_api_key`,
	Args: cobra.ExactArgs(1),
	RunE: getConfigKey,
}

func init() {
	configGetCmd.Flags().Bool("show-sensitive", false, "Do not redact sensitive values")
}

func getConfigKey(cmd *cobra.Command, args []string) error {
	key := args[0]

	if !viper.IsSet(key) {
		return fmt.Errorf("key not found: %s", key)
	}

	value := viper.Get(key)

	showSensitive, _ := cmd.Flags().GetBool("show-sensitive")
	if isSensitive(key) && !showSensitive {
		value = "[REDACTED]"
	}

	fmt.Fprintf(cmd.OutOrStdout(), "%v\n", value)
	return nil
}
