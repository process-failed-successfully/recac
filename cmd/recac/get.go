package main

import (
	"fmt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
)

var getCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a configuration value",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		key := args[0]
		value := viper.Get(key)
		if value == nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error: key not found: %s\n", key)
			os.Exit(1)
		}
		fmt.Fprintln(cmd.OutOrStdout(), value)
	},
}

func init() {
	configCmd.AddCommand(getCmd)
}
