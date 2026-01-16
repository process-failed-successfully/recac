package main

import (
	"flag"
	"fmt"
	"os"
	"recac/internal/config"
	"recac/internal/telemetry"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var exit = os.Exit
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
			exit(1)
		}
	}()

	fmt.Fprintln(os.Stderr, "WARNING: The 'recac' binary is deprecated and will be removed in a future release.")
	fmt.Fprintln(os.Stderr, "Please use 'orchestrator' or 'recac-agent' (agent) binaries instead.")

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: command not found: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run 'recac --help' for usage.")
		exit(1)
	}
}

func init() {
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		// Default behavior: Run Interactive Mode using flags
		provider, _ := cmd.Flags().GetString("provider")
		model, _ := cmd.Flags().GetString("model")
		RunInteractive(provider, model)
	}
	cobra.OnInitialize(initConfig)

	// Initialize commands
	initHistoryCmd(rootCmd)
	initHintCmd(rootCmd)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.

	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.recac.yaml)")
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose/debug logging")
	rootCmd.PersistentFlags().String("model", "", "Model to use (overrides config and RECAC_MODEL env var)")
	rootCmd.PersistentFlags().String("provider", "", "Agent provider (gemini, openai, openrouter, etc)")
	rootCmd.PersistentFlags().Bool("mock", false, "Start in mock mode (no Docker or API keys required)")

	viper.BindPFlag("verbose", rootCmd.PersistentFlags().Lookup("verbose"))
	viper.BindPFlag("model", rootCmd.PersistentFlags().Lookup("model"))
	viper.BindPFlag("provider", rootCmd.PersistentFlags().Lookup("provider"))
	viper.BindPFlag("mock", rootCmd.PersistentFlags().Lookup("mock"))

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	config.Load(cfgFile)

	// Validate configuration values
	if err := config.ValidateConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exit(1)
	}

	telemetry.InitLogger(viper.GetBool("verbose"), "", false)

	// Start Metrics Server, but not in test mode to avoid hanging
	if flag.Lookup("test.v") == nil {
		go func() {
			port := viper.GetInt("metrics_port")
			if err := telemetry.StartMetricsServer(port); err != nil {
				fmt.Fprintf(os.Stderr, "Warning: Failed to start metrics server: %v\n", err)
			}
		}()
	}
}
