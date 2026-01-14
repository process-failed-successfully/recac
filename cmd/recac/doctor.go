package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"syscall"

	"recac/internal/docker"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var autoFixFlag bool

// doctorCmd represents the doctor command
var doctorCmd = &cobra.Command{
	Use:          "doctor",
	Short:        "Diagnose potential issues with the environment",
	SilenceUsage: true, // Prevents printing usage on error
	Long: `The doctor command runs a series of checks to verify that the environment
is correctly configured. It checks for Docker, configuration, and API connectivity.`,
	RunE: runChecks,
}

// runChecks executes all the doctor checks and prints a summary.
func runChecks(cmd *cobra.Command, args []string) error {
	out := cmd.OutOrStdout()
	checkPassed := true

	fmt.Fprintln(out, "🩺 Running doctor checks...")

	// Docker checks
	if !runDockerChecks(cmd) {
		checkPassed = false
	}

	// Config checks
	if !runConfigChecks(cmd) {
		checkPassed = false
	}

	// Summary
	fmt.Fprintln(out, "\n🩺 Doctor Summary:")
	if checkPassed {
		fmt.Fprintln(out, "✅ All checks passed!")
		return nil
	}

	fmt.Fprintln(out, "❌ Some checks failed. Please review the output above.")
	return fmt.Errorf("doctor checks failed")
}

// runDockerChecks runs all Docker-related checks.
func runDockerChecks(cmd *cobra.Command) bool {
	out := cmd.OutOrStdout()
	ctx := context.Background()
	allChecksPassed := true
	autoFixAttempted := false

	// Step 1: Check Docker daemon connectivity
	fmt.Fprintln(out, "\n🔎 Checking Docker daemon connectivity...")
	client, err := docker.NewClient("check-docker")
	if err != nil {
		fmt.Fprintf(out, "❌ Error creating docker client: %v\n", err)
		return false
	}
	defer client.Close()

	daemonCheckPassed := true
	if err := client.CheckDaemon(ctx); err != nil {
		fmt.Fprintf(out, "❌ Docker daemon is not reachable: %v\n", err)
		allChecksPassed = false
		daemonCheckPassed = false
	} else {
		fmt.Fprintln(out, "✅ Docker daemon is reachable")
	}

	// Step 2: Check Docker socket accessibility
	fmt.Fprintln(out, "\n🔎 Checking Docker socket accessibility...")
	socketCheckPassed := true
	if err := client.CheckSocket(ctx); err != nil {
		fmt.Fprintf(out, "❌ Docker socket is not accessible: %v\n", err)
		allChecksPassed = false
		socketCheckPassed = false
	} else {
		fmt.Fprintln(out, "✅ Docker socket is accessible")
	}

	// Step 3: Check disk space (if auto-fix enabled)
	if autoFixFlag {
		fmt.Fprintln(out, "\n🔎 Checking disk space...")
		diskSpaceOK, err := checkDiskSpace()
		if err != nil {
			fmt.Fprintf(out, "⚠️  Could not check disk space: %v\n", err)
		} else if !diskSpaceOK {
			fmt.Fprintln(out, "⚠️  Low disk space detected. Consider freeing up space.")
		} else {
			fmt.Fprintln(out, "✅ Sufficient disk space available")
		}
	}

	// Step 4: Check required Docker images
	fmt.Fprintln(out, "\n🔎 Checking required Docker images...")
	requiredImages := []string{"ubuntu:latest"}
	allImagesPresent := true
	missingImages := []string{}
	for _, imageRef := range requiredImages {
		exists, err := client.CheckImage(ctx, imageRef)
		if err != nil {
			fmt.Fprintf(out, "❌ Error checking image %s: %v\n", imageRef, err)
			allImagesPresent = false
			allChecksPassed = false
		} else if !exists {
			fmt.Fprintf(out, "⚠️  Required image %s is not present locally\n", imageRef)
			allImagesPresent = false
			missingImages = append(missingImages, imageRef)
		} else {
			fmt.Fprintf(out, "✅ Required image %s is present\n", imageRef)
		}
	}

	// Auto-fix: Pull missing images
	if autoFixFlag && len(missingImages) > 0 && daemonCheckPassed && socketCheckPassed {
		fmt.Fprintln(out, "\n=== Auto-fix: Pulling missing images ===")
		autoFixAttempted = true
		for _, imageRef := range missingImages {
			fmt.Fprintf(out, "🔧 Attempting to pull missing image: %s\n", imageRef)
			fmt.Fprintf(out, "📥 Pulling image %s...\n", imageRef)
			if err := client.PullImage(ctx, imageRef); err != nil {
				fmt.Fprintf(out, "❌ Auto-fix failed to pull %s: %v\n", imageRef, err)
				allChecksPassed = false
			} else {
				fmt.Fprintf(out, "✅ Auto-fix successfully pulled %s\n", imageRef)
				allImagesPresent = true
			}
		}
	}

	// Summary
	fmt.Fprintln(out, "\n=== Docker Check Summary ===")
	if autoFixAttempted {
		fmt.Fprintln(out, "🔧 Auto-fix was attempted")
	}
	if allChecksPassed && allImagesPresent {
		fmt.Fprintln(out, "✅ All Docker checks passed: Docker is available and ready")
		return true
	} else if allChecksPassed {
		if autoFixFlag {
			fmt.Fprintln(out, "⚠️  Docker is available, but some required images could not be pulled")
		} else {
			fmt.Fprintln(out, "⚠️  Docker is available, but some required images are missing")
			fmt.Fprintln(out, "   Run with --auto-fix to automatically pull missing images")
		}
		return true // Still counts as a "pass" for the doctor command
	} else {
		fmt.Fprintln(out, "❌ Docker is not available or not properly configured")
		if autoFixFlag && autoFixAttempted {
			fmt.Fprintln(out, "   Auto-fix was attempted but some issues could not be resolved")
		}
		return false
	}
}

// checkDiskSpace checks if there's sufficient disk space available.
// Returns true if disk space is OK (> 1GB free), false otherwise.
func checkDiskSpace() (bool, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err != nil {
		return false, err
	}

	// Calculate free space in bytes
	freeBytes := stat.Bavail * uint64(stat.Bsize)
	// Convert to GB (1GB = 1024^3 bytes)
	freeGB := float64(freeBytes) / (1024 * 1024 * 1024)

	// Check if we have at least 1GB free
	if freeGB < 1.0 {
		return false, nil
	}

	return true, nil
}

func init() {
	doctorCmd.Flags().BoolVar(&autoFixFlag, "auto-fix", false, "Automatically attempt to fix issues (pull missing images, check disk space)")
	rootCmd.AddCommand(doctorCmd)
}



// runConfigChecks runs all configuration-related checks.
func runConfigChecks(cmd *cobra.Command) bool {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "\n🔎 Checking configuration...")
	allChecksPassed := true

	// Check for config file existence
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(out, "❌ Could not determine home directory")
		return false
	}
	configFile := filepath.Join(home, ".recac.yaml")
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		fmt.Fprintf(out, "❌ Config file not found at %s\n", configFile)
		return false
	} else {
		fmt.Fprintf(out, "✅ Config file found at %s\n", configFile)
	}

	// Check for required keys
	if !viper.IsSet("agent_provider") {
		fmt.Fprintln(out, "❌ Missing required key 'agent_provider' in config")
		allChecksPassed = false
	} else {
		fmt.Fprintln(out, "✅ Required key 'agent_provider' is set")
	}

	if !viper.IsSet("api_key") {
		fmt.Fprintln(out, "❌ Missing required key 'api_key' in config")
		allChecksPassed = false
	} else {
		fmt.Fprintln(out, "✅ Required key 'api_key' is set")
	}

	return allChecksPassed
}
