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

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "recac",
	Short: "RECAC: Combined Autonomous Coding (Go Refactor)",
	Long: `RECAC is a premium, type-safe, and high-performance autonomous coding framework 
re-implemented in Go. It provides a world-class CLI/TUI experience for 
managing autonomous coding sessions.`,
	SilenceErrors: true,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	// Wrap Execute in panic recovery for graceful shutdown
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "\n=== CRITICAL ERROR: Command Execution Panic ===\n")
			fmt.Fprintf(os.Stderr, "Error: %v\n", r)
			fmt.Fprintf(os.Stderr, "Attempting graceful shutdown...\n")
			os.Exit(1)
		}
	}()

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: command not found: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run 'recac --help' for usage.")
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.recac.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose/debug logging")
	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Search config in home directory with name ".recac" (without extension).
		viper.AddConfigPath(".")
		viper.SetConfigType("yaml")
		viper.SetConfigName("config")
	}

	viper.SetEnvPrefix("RECAC")
	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}

	// Validate configuration values
	if err := config.ValidateConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	telemetry.InitLogger(viper.GetBool("verbose"))
}
