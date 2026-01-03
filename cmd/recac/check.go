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
	Use:   "check",
	Short: "Check dependencies and environment",
	Long: `Perform pre-flight checks on the environment and dependencies.
Use --fix to automatically attempt repairs for minor issues.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Running pre-flight checks...")
		allPassed := true

		// 1. Check Config
		if err := checkConfig(); err != nil {
			allPassed = false
			fmt.Printf("❌ Config: %v\n", err)
			if fixFlag {
				if err := fixConfig(); err != nil {
					fmt.Printf("  Failed to fix config: %v\n", err)
				} else {
					fmt.Printf("  ✅ Config fixed (created default)\n")
					allPassed = true // reset? strictly speaking no, but for flow
				}
			}
		} else {
			fmt.Println("✅ Config found")
		}

		// 2. Check Go
		if err := checkGo(); err != nil {
			allPassed = false
			fmt.Printf("❌ Go: %v\n", err)
		} else {
			fmt.Println("✅ Go installed")
		}

		// 3. Check Docker
		if err := checkDocker(); err != nil {
			allPassed = false
			fmt.Printf("❌ Docker: %v\n", err)
		} else {
			fmt.Println("✅ Docker running")
		}

		if allPassed {
			fmt.Println("\nAll checks passed! 🚀")
		} else {
			fmt.Println("\nSome checks failed.")
			if !fixFlag {
				fmt.Println("Run with --fix to attempt automatic repairs.")
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
	return viper.SafeWriteConfig()
}

func checkGo() error {
	_, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go binary not found in PATH")
	}
	return nil
}

func checkDocker() error {
	cmd := exec.Command("docker", "info")
	return cmd.Run()
}
