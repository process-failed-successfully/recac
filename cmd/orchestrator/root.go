package main

import (
	"fmt"
	"os"
	"recac/internal/config"
	"recac/internal/telemetry"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

var rootCmd = &cobra.Command{
	Use:   "orchestrator",
	Short: "RECAC Orchestrator",
	Long:  `The Orchestrator for the RECAC distributed autonomous coding system.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.recac.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose/debug logging")
	rootCmd.PersistentFlags().String("work-file", "work_items.json", "Work items file (for 'file' poller and task management)")

	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("orchestrator.work_file", rootCmd.PersistentFlags().Lookup("work-file"))
	viper.BindEnv("orchestrator.work_file", "RECAC_WORK_FILE")
}

func initConfig() {
	config.Load(cfgFile)
	telemetry.InitLogger(viper.GetBool("verbose"), "orchestrator", false)
}
