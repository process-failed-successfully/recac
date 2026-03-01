package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var configEditCmd = &cobra.Command{
	Use:   "edit",
	Short: "Edit the configuration file in your default editor",
	Long: `Edit the application's configuration file using your default editor.

It uses the $EDITOR environment variable. If not set, it defaults to 'vim'.
If no configuration file exists, it will prompt you to create one.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configFile := viperConfigFileUsed()
		if configFile == "" {
			configFile = "config.yaml"
			fmt.Fprintf(cmd.OutOrStdout(), "No configuration file found. We will create %s for you.\n", configFile)
			if err := viper.WriteConfigAs(configFile); err != nil {
				return fmt.Errorf("failed to create config file %s: %w", configFile, err)
			}
		}

		editor := os.Getenv("EDITOR")
		if editor == "" {
			editor = "vim" // Fallback
		}

		c := execCommand(editor, configFile)
		c.Stdin = os.Stdin
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		if err := c.Run(); err != nil {
			return fmt.Errorf("failed to open editor %s: %w", editor, err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Configuration saved.\n")
		return nil
	},
}

func init() {
	configCmd.AddCommand(configEditCmd)
}
