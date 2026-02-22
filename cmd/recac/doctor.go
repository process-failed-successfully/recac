package main

import (
	"fmt"
	"os"
	"recac/internal/cmdutils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	doctorCmd.Flags().Bool("fix", false, "Attempt to fix issues automatically")
	rootCmd.AddCommand(doctorCmd)
}

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Check the RECAC environment for potential issues",
	Long:  `Runs a series of checks to ensure that the RECAC environment is set up correctly. This includes checking for a valid configuration file, required dependencies like git and docker, and connectivity to the Docker daemon.`,
	Run: func(cmd *cobra.Command, args []string) {
		fix, _ := cmd.Flags().GetBool("fix")
		if fix {
			if viper.ConfigFileUsed() == "" {
				if err := cmdutils.FixConfig(); err != nil {
					fmt.Printf("Failed to fix config: %v\n", err)
				} else {
					fmt.Println("Config fixed (created default)")
				}
			}
		}

		report, passed := cmdutils.GetDoctor()
		fmt.Fprint(cmd.OutOrStdout(), report)

		if !passed {
			os.Exit(1)
		}
	},
}
