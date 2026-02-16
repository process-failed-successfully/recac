package main

import (
	"fmt"
	"os"

	"recac/internal/config"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile string
	verbose bool
)

var rootCmd = &cobra.Command{
	Use:   "orchestrator",
	Short: "RECAC Orchestrator",
	Long:  `The Orchestrator component of RECAC, responsible for polling tasks and spawning agents.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		config.Load(cfgFile)
	},
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
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
}
