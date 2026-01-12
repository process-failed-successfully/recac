package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var fixFlag bool

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:     "check",
	Short:   "DEPRECATED: Use the 'doctor' command instead.",
	Long:    `DEPRECATED: This command is no longer supported. Please use 'recac doctor'.`,
	Hidden:  true, // Hide it from the help command
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Fprintln(os.Stderr, "Warning: The 'check' command is deprecated and will be removed in a future release. Use 'recac doctor' instead.")
		// Forward the call to the doctor command
		return doctorCmd.RunE(cmd, args)
	},
}

func init() {
	rootCmd.AddCommand(checkCmd)
}

var checkConfig = func() error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("config file not found")
	}
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file %s does not exist", configFile)
	}
	return nil
}

var checkGo = func() error {
	_, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go binary not found in PATH")
	}
	return nil
}

var checkDocker = func() error {
	cmd := exec.Command("docker", "info")
	return cmd.Run()
}
