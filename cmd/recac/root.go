package main

import (
	"fmt"
	"os"
	"recac/internal/config"
	"recac/internal/telemetry"
	"strings"

	"github.com/joho/godotenv"
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

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: command not found: %v\n", err)
		fmt.Fprintln(os.Stderr, "Run 'recac --help' for usage.")
		exit(1)
	}
}

func init() {
	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		// Default behavior: Run Interactive Mode
		RunInteractive()
	}
	cobra.OnInitialize(initConfig)

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
	// explicit .env loading
	if err := godotenv.Load(); err != nil {
		// handle error if you want, or ignore if .env is missing
		// fmt.Println("No .env file found")
	}

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
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv() // read in environment variables that match

	// Check for standard JIRA_URL if RECAC_JIRA_URL is not set
	if os.Getenv("RECAC_JIRA_URL") == "" && os.Getenv("JIRA_URL") != "" {
		viper.SetDefault("jira.url", os.Getenv("JIRA_URL"))
	}

	// Set defaults
	viper.SetDefault("provider", "gemini")
	viper.SetDefault("model", "gemini-pro")
	viper.SetDefault("max_iterations", 20)
	viper.SetDefault("manager_frequency", 5)
	viper.SetDefault("timeout", 300)
	viper.SetDefault("docker_timeout", 600)
	viper.SetDefault("bash_timeout", 600)
	viper.SetDefault("agent_timeout", 300)
	viper.SetDefault("metrics_port", 2112)
	viper.SetDefault("verbose", false)
	viper.SetDefault("git_user_email", "recac-agent@example.com")
	viper.SetDefault("git_user_name", "RECAC Agent")

	// Notification Defaults
	slackEnabled := false
	if os.Getenv("SLACK_BOT_USER_TOKEN") != "" {
		slackEnabled = true
	}
	viper.SetDefault("notifications.slack.enabled", slackEnabled)
	viper.SetDefault("notifications.slack.channel", "#general")
	viper.SetDefault("notifications.slack.events.on_start", true)
	viper.SetDefault("notifications.slack.events.on_success", true)
	viper.SetDefault("notifications.slack.events.on_failure", true)
	viper.SetDefault("notifications.slack.events.on_user_interaction", true)
	viper.SetDefault("notifications.slack.events.on_project_complete", true)

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	} else {
		// Config file not found; create one with defaults
		if _, ok := err.(viper.ConfigFileNotFoundError); ok || true { // Force creation if failing to read for any reason (simplification)
			// check if we already tried to read a specific file
			if cfgFile == "" {
				// Write config to current directory
				viper.SetConfigName("config")
				viper.SetConfigType("yaml")
				viper.AddConfigPath(".")

				// Attempt to write
				if err := viper.SafeWriteConfig(); err != nil {
					// Check if it already exists (SafeWriteConfig fails if exists)
					// But ReadInConfig failed, so likely it doesn't exist or is invalid.
					// If it exists but is invalid, we probably shouldn't overwrite it blindly,
					// but the requirement is "When first run... create the config file".
					if _, err := os.Stat("config.yaml"); os.IsNotExist(err) {
						if err := viper.WriteConfigAs("config.yaml"); err != nil {
							fmt.Fprintf(os.Stderr, "Warning: Failed to create default config file: %v\n", err)
						} else {
							fmt.Println("Created default configuration file: config.yaml")
						}
					}
				} else {
					fmt.Println("Created default configuration file: config.yaml")
				}
			}
		}
	}

	// Validate configuration values
	if err := config.ValidateConfig(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		exit(1)
	}

	telemetry.InitLogger(viper.GetBool("verbose"), "")

	// Start Metrics Server
	go func() {
		port := viper.GetInt("metrics_port")
		if err := telemetry.StartMetricsServer(port); err != nil {
			fmt.Fprintf(os.Stderr, "Warning: Failed to start metrics server: %v\n", err)
		}
	}()
}
