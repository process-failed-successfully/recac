package main

import (
	"fmt"
	"recac/internal/config"

	"github.com/spf13/cobra"
)

var configValidateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate the configuration",
	Long: `Validate the configuration file and environment variables for the recac CLI.

This command explicitly checks if the current configuration contains valid values
(e.g., positive timeouts, valid port numbers). If the configuration is valid,
it will output a success message. If it is invalid, it will print the validation
errors and exit with a non-zero status code.

Example:
  recac config validate`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		err := config.ValidateConfig()
		if err != nil {
			return err
		}

		fmt.Fprintln(cmd.OutOrStdout(), "Configuration is valid.")
		return nil
	},
}

func init() {
	configCmd.AddCommand(configValidateCmd)
}
