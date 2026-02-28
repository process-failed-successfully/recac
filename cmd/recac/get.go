package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var getCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		key := args[0]
		value := viper.Get(key)
		if value == nil {
			return fmt.Errorf("key not found: %s", key)
		}
		fmt.Fprintln(cmd.OutOrStdout(), value)
		return nil
	},
}

func init() {
	configCmd.AddCommand(getCmd)
}
