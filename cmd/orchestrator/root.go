package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var cfgFile string
var verbose bool

var rootCmd = &cobra.Command{
	Use:   "orchestrator",
	Short: "Distributed Orchestrator for RECAC",
	Long: `The Orchestrator manages the lifecycle of autonomous coding agents.
It polls for work items (Jira, GitHub, Files) and spawns agents (Docker, K8s).`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.recac.yaml)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose/debug logging")
}
