package main

import (
	"fmt"
	"recac/internal/ui"

	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the RECAC environment for potential issues",
	Long:  `Runs a series of checks to ensure that the RECAC environment is set up correctly. This includes checking for a valid configuration file, required dependencies like git and docker, and connectivity to the Docker daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprint(cmd.OutOrStdout(), ui.GetDoctor())
	},
}
