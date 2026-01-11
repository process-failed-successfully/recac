package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"recac/internal/docker"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var fixFlag bool

// dockerChecker defines the interface for Docker checks, allowing for mocking.
type dockerChecker interface {
	CheckDaemon(context.Context) error
	CheckSocket(context.Context) error
	CheckImage(context.Context, string) (bool, error)
	PullImage(context.Context, string) error
	Close() error
}

// newDockerClient is a factory function for the Docker client.
// It can be overridden in tests to inject a mock client.
var newDockerClient = func(component string) (dockerChecker, error) {
	return docker.NewClient(component)
}

// checkCmd represents the check command
var checkCmd = &cobra.Command{
	Use:   "check",
	Short: "Check dependencies and environment",
	Long: `Perform pre-flight checks on the environment and dependencies.
Use --fix to automatically attempt repairs for minor issues.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("Running pre-flight checks...")
		var checksFailed bool
		var finalErrorMessages []string

		// 1. Check Config
		if err := checkConfig(); err != nil {
			checksFailed = true
			msg := fmt.Sprintf("❌ Config: %v", err)
			fmt.Println(msg)
			finalErrorMessages = append(finalErrorMessages, msg)
			if fixFlag {
				fmt.Println("🔧 Attempting to fix missing config file...")
				if err := fixConfig(); err != nil {
					fmt.Printf("  Failed to fix config: %v\n", err)
				} else {
					fmt.Println("  ✅ Config fixed (created default)")
				}
			}
		} else {
			fmt.Println("✅ Config found")
		}

		// 2. Check Git
		if err := checkGit(); err != nil {
			checksFailed = true
			msg := fmt.Sprintf("❌ Git: %v", err)
			fmt.Println(msg)
			finalErrorMessages = append(finalErrorMessages, msg)
		} else {
			fmt.Println("✅ Git installed")
		}

		// 3. Check Go
		if err := checkGo(); err != nil {
			checksFailed = true
			msg := fmt.Sprintf("❌ Go: %v", err)
			fmt.Println(msg)
			finalErrorMessages = append(finalErrorMessages, msg)
		} else {
			fmt.Println("✅ Go installed")
		}

		// 4. Check Docker
		dockerClient, err := newDockerClient("check")
		if err != nil {
			checksFailed = true
			msg := fmt.Sprintf("❌ Docker: Failed to create client: %v", err)
			fmt.Println(msg)
			finalErrorMessages = append(finalErrorMessages, msg)
		} else {
			defer dockerClient.Close()
			if errs := checkDocker(dockerClient); len(errs) > 0 {
				checksFailed = true
				fmt.Println("❌ Docker: docker setup is incomplete.")
				for _, e := range errs {
					msg := fmt.Sprintf("  %v", e)
					fmt.Println(msg)
					finalErrorMessages = append(finalErrorMessages, e.Error())
				}

				if fixFlag {
					fmt.Println("🔧 Attempting to pull missing images...")
					if err := fixDocker(dockerClient, errs); err != nil {
						fmt.Printf("  Failed to fix docker issues: %v\n", err)
					}
				}
			} else {
				fmt.Println("✅ Docker is available and ready")
			}
		}

		if !checksFailed {
			fmt.Println("\nAll checks passed! 🚀")
			return nil
		}

		fmt.Println("\nSome checks failed.")
		if !fixFlag {
			fmt.Println("Run with --fix to attempt automatic repairs.")
		}
		return errors.New(strings.Join(finalErrorMessages, "; "))
	},
}

func init() {
	checkCmd.Flags().BoolVar(&fixFlag, "fix", false, "Attempt to fix issues automatically")
	rootCmd.AddCommand(checkCmd)
}

func checkConfig() error {
	// A simple check is to see if any config is loaded.
	if len(viper.AllKeys()) == 0 {
		return fmt.Errorf("no config file found or is empty")
	}
	return nil
}

func fixConfig() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	configPath := home + "/.recac"
	configFile := configPath + "/config.yaml"

	if err := os.MkdirAll(configPath, 0755); err != nil {
		return err
	}

	viper.Set("agent_provider", "gemini")
	viper.Set("agent_model", "gemini-pro")
	return viper.WriteConfigAs(configFile)
}

func checkGo() error {
	_, err := exec.LookPath("go")
	if err != nil {
		return fmt.Errorf("go binary not found in PATH")
	}
	return nil
}

func checkGit() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("git binary not found in PATH")
	}
	return nil
}

func checkDocker(client dockerChecker) []error {
	var errs []error
	ctx := context.Background()

	// Check 1: Daemon Reachable
	if err := client.CheckDaemon(ctx); err != nil {
		errs = append(errs, errors.New("❌ Docker daemon is not reachable"))
	}

	// Check 2: Socket Accessible
	if err := client.CheckSocket(ctx); err != nil {
		errs = append(errs, errors.New("❌ Docker socket is not accessible"))
	}

	// Check 3: Base Image Exists
	requiredImage := "ubuntu:latest"
	hasImage, err := client.CheckImage(ctx, requiredImage)
	if err != nil {
		errs = append(errs, fmt.Errorf("❌ Error checking for image %s: %w", requiredImage, err))
	} else if !hasImage {
		errs = append(errs, fmt.Errorf("❌ Required Docker image not found: %s", requiredImage))
	}

	return errs
}

func fixDocker(client dockerChecker, errs []error) error {
	ctx := context.Background()
	requiredImage := "ubuntu:latest"
	needsPull := false
	for _, err := range errs {
		if strings.Contains(err.Error(), requiredImage) {
			needsPull = true
			break
		}
	}

	if needsPull {
		fmt.Printf("  Pulling %s...\n", requiredImage)
		if err := client.PullImage(ctx, requiredImage); err != nil {
			return fmt.Errorf("failed to pull image %s: %w", requiredImage, err)
		}
		fmt.Printf("  ✅ Auto-fix successfully pulled %s\n", requiredImage)
	}

	return nil
}
