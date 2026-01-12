package main

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var doctorCmd = &cobra.Command{
	Use:   "doctor",
	Short: "Run a full diagnostic check of the recac environment",
	Long: `The doctor command runs a comprehensive suite of checks to ensure that
the recac environment is correctly configured. It verifies configuration,
Docker functionality, API connectivity, and more.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🩺 Running diagnostics...")
		allPassed := true

		// Check 1: Configuration
		if err := checkConfigDoctor(); err != nil {
			fmt.Printf("❌ Configuration: %v\n", err)
			allPassed = false
		} else {
			fmt.Printf("✅ Configuration: OK (%s)\n", viper.ConfigFileUsed())
		}

		// Check 2: Docker
		if err := checkDockerDoctor(); err != nil {
			fmt.Printf("❌ Docker: %v\n", err)
			allPassed = false
		} else {
			fmt.Println("✅ Docker: OK")
		}

		// Check 3: API Connectivity (placeholder)
		fmt.Println("🟡 API Connectivity: (Not yet implemented)")

		// Check 4: Jira Connectivity (placeholder)
		fmt.Println("🟡 Jira Connectivity: (Not yet implemented)")


		if allPassed {
			fmt.Println("\n✨ Your environment looks good to go! ✨")
		} else {
			fmt.Println("\nSome checks failed. Please review the output above.")
			exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(doctorCmd)
}

func checkConfigDoctor() error {
	configFile := viper.ConfigFileUsed()
	if configFile == "" {
		return fmt.Errorf("no config file found. Please run 'recac init'")
	}
	return nil
}

func checkDockerDoctor() error {
	cmd := exec.Command("docker", "info")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Docker daemon is not running or docker is not installed")
	}

	// More advanced check: run a container
	cmd = exec.Command("docker", "run", "--rm", "hello-world")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run 'hello-world' container: %s", string(output))
	}
	return nil
}
