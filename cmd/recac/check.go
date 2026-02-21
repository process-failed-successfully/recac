package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var fixFlag bool

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check dependencies and environment",
	Long: `Perform pre-flight checks on the environment and dependencies.
Use --fix to automatically attempt repairs for minor issues.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Fprintln(cmd.OutOrStdout(), "Running pre-flight checks...")
		allPassed := true

		// 1. Check Config
		if err := checkConfig(); err != nil {
			allPassed = false
			fmt.Fprintf(cmd.ErrOrStderr(), "❌ Config: %v\n", err)
			if fixFlag {
				if err := fixConfig(); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "  Failed to fix config: %v\n", err)
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "  ✅ Config fixed (created default)\n")
					allPassed = true // reset? strictly speaking no, but for flow
				}
			}
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ Config found")
		}

		// 2. Check Go
		if err := checkGo(); err != nil {
			allPassed = false
			fmt.Fprintf(cmd.ErrOrStderr(), "❌ Go: %v\n", err)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ Go installed")
		}

		// 3. Check Docker
		if err := checkDocker(); err != nil {
			allPassed = false
			fmt.Fprintf(cmd.ErrOrStderr(), "❌ Docker: %v\n", err)
		} else {
			fmt.Fprintln(cmd.OutOrStdout(), "✅ Docker running")
		}

		if allPassed {
			fmt.Fprintln(cmd.OutOrStdout(), "\nAll checks passed! 🚀")
		} else {
			fmt.Fprintln(cmd.ErrOrStderr(), "\nSome checks failed.")
			if !fixFlag {
				fmt.Fprintln(cmd.ErrOrStderr(), "Run with --fix to attempt automatic repairs.")
			}
			exit(1)
		}
	},
}

func init() {
	checkCmd.Flags().BoolVar(&fixFlag, "fix", false, "Attempt to fix issues automatically")
	rootCmd.AddCommand(checkCmd)
}

func checkConfig() error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("config file not found")
	}
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		return fmt.Errorf("config file %s does not exist", configFile)
	}
	return nil
}

func fixConfig() error {
	// Simple fix: create default config if missing
	viper.SetDefault("provider", "gemini")
	viper.SetDefault("model", "gemini-pro")

	if viper.ConfigFileUsed() != "" {
		return viper.SafeWriteConfig()
	}

	// Fallback to default location
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	target := filepath.Join(home, ".recac.yaml")
	return viper.SafeWriteConfigAs(target)
}

func checkGo() error {
	_, err := execLookPath("go")
	if err != nil {
		return fmt.Errorf("go binary not found in PATH")
	}
	return nil
}

func checkDocker() error {
	cmd := execCommand("docker", "info")
	return cmd.Run()
}
