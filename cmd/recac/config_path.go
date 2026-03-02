package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Print the path to the configuration file",
	Long: `Print the path to the configuration file that is currently being used by the recac CLI.

Example:
  recac config path`,
	Args: cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfgFile := viperConfigFileUsed()
		if cfgFile == "" {
			cfgFile = "config.yaml"
		}

		fmt.Fprintln(cmd.OutOrStdout(), cfgFile)
		return nil
	},
}

func init() {
	configCmd.AddCommand(configPathCmd)
}
