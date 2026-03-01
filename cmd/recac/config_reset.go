package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configResetForce bool

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset the configuration file to its default state",
	Long: `Reset the configuration file to its default state.

This will overwrite your existing configuration file, removing all custom settings,
including API keys, models, and UI preferences.

Examples:
  recac config reset
  recac config reset --force`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgFile := viperConfigFileUsed()
		if cfgFile == "" {
			cfgFile = "config.yaml"
		}

		if !configResetForce {
			fmt.Fprintf(cmd.OutOrStdout(), "WARNING: This will delete all your configuration settings in %s.\n", cfgFile)
			fmt.Fprintf(cmd.OutOrStdout(), "Are you sure you want to proceed? [y/N]: ")

			scanner := bufio.NewScanner(cmd.InOrStdin())
			if scanner.Scan() {
				input := strings.ToLower(strings.TrimSpace(scanner.Text()))
				if input != "y" && input != "yes" {
					fmt.Fprintf(cmd.OutOrStdout(), "Reset cancelled.\n")
					return nil
				}
			} else {
				// EOF or error
				return nil
			}
		}

		// Delete the file if it exists
		if _, err := os.Stat(cfgFile); err == nil {
			if err := os.Remove(cfgFile); err != nil {
				return fmt.Errorf("failed to delete configuration file %s: %w", cfgFile, err)
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("failed to check configuration file %s: %w", cfgFile, err)
		}

		// Reset Viper's state
		viper.Reset()

		// Set the file again in case viper.WriteConfigAs needs it (though WriteConfigAs specifies the path)
		viper.SetConfigFile(cfgFile)

		// Create a new, empty config file by writing the default (empty) viper config
		if err := viper.WriteConfigAs(cfgFile); err != nil {
			return fmt.Errorf("failed to write new configuration file %s: %w", cfgFile, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Successfully reset configuration.\n")
		return nil
	},
}

func init() {
	configResetCmd.Flags().BoolVarP(&configResetForce, "force", "f", false, "Force reset without prompting for confirmation")
	configCmd.AddCommand(configResetCmd)
}
