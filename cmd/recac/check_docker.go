package main

import (
	"context"
	"fmt"
	"syscall"

	"recac/internal/docker"

	"github.com/spf13/cobra"
)

var autoFixFlag bool

// checkDockerCmd represents the check-docker command
var checkDockerCmd = &cobra.Command{
	Use:   "check-docker",
	Short: "Check if Docker daemon is running",
	Long:  `Check Docker daemon connectivity, socket accessibility, and required images.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()
		allChecksPassed := true
		autoFixAttempted := false

		// Step 1: Check Docker daemon connectivity
		fmt.Println("Checking Docker daemon connectivity...")
		client, err := docker.NewClient("check-docker")
		if err != nil {
			fmt.Printf("❌ Error creating docker client: %v\n", err)
			exit(1)
		}
		defer client.Close()

		daemonCheckPassed := true
		if err := client.CheckDaemon(ctx); err != nil {
			fmt.Printf("❌ Docker daemon is not reachable: %v\n", err)
			allChecksPassed = false
			daemonCheckPassed = false
		} else {
			fmt.Println("✅ Docker daemon is reachable")
		}

		// Step 2: Check Docker socket accessibility
		fmt.Println("\nChecking Docker socket accessibility...")
		socketCheckPassed := true
		if err := client.CheckSocket(ctx); err != nil {
			fmt.Printf("❌ Docker socket is not accessible: %v\n", err)
			allChecksPassed = false
			socketCheckPassed = false
		} else {
			fmt.Println("✅ Docker socket is accessible")
		}

		// Step 3: Check disk space (if auto-fix enabled)
		if autoFixFlag {
			fmt.Println("\nChecking disk space...")
			diskSpaceOK, err := checkDiskSpace()
			if err != nil {
				fmt.Printf("⚠️  Could not check disk space: %v\n", err)
			} else if !diskSpaceOK {
				fmt.Printf("⚠️  Low disk space detected. Consider freeing up space.\n")
			} else {
				fmt.Println("✅ Sufficient disk space available")
			}
		}

		// Step 4: Check required Docker images
		fmt.Println("\nChecking required Docker images...")
		requiredImages := []string{"ubuntu:latest"}
		allImagesPresent := true
		missingImages := []string{}
		for _, imageRef := range requiredImages {
			exists, err := client.CheckImage(ctx, imageRef)
			if err != nil {
				fmt.Printf("❌ Error checking image %s: %v\n", imageRef, err)
				allImagesPresent = false
				allChecksPassed = false
			} else if !exists {
				fmt.Printf("⚠️  Required image %s is not present locally\n", imageRef)
				allImagesPresent = false
				missingImages = append(missingImages, imageRef)
			} else {
				fmt.Printf("✅ Required image %s is present\n", imageRef)
			}
		}

		// Auto-fix: Pull missing images
		if autoFixFlag && len(missingImages) > 0 && daemonCheckPassed && socketCheckPassed {
			fmt.Println("\n=== Auto-fix: Pulling missing images ===")
			autoFixAttempted = true
			for _, imageRef := range missingImages {
				fmt.Printf("🔧 Attempting to pull missing image: %s\n", imageRef)
				fmt.Printf("📥 Pulling image %s...\n", imageRef)
				if err := client.PullImage(ctx, imageRef); err != nil {
					fmt.Printf("❌ Auto-fix failed to pull %s: %v\n", imageRef, err)
					allChecksPassed = false
				} else {
					fmt.Printf("✅ Auto-fix successfully pulled %s\n", imageRef)
					allImagesPresent = true
				}
			}
		}

		// Summary
		fmt.Println("\n=== Docker Check Summary ===")
		if autoFixAttempted {
			fmt.Println("🔧 Auto-fix was attempted")
		}
		if allChecksPassed && allImagesPresent {
			fmt.Println("✅ All checks passed: Docker is available and ready")
			exit(0)
		} else if allChecksPassed {
			if autoFixFlag {
				fmt.Println("⚠️  Docker is available, but some required images could not be pulled")
			} else {
				fmt.Println("⚠️  Docker is available, but some required images are missing")
				fmt.Println("   Run with --auto-fix to automatically pull missing images")
			}
			exit(0)
		} else {
			fmt.Println("❌ Docker is not available or not properly configured")
			if autoFixFlag && autoFixAttempted {
				fmt.Println("   Auto-fix was attempted but some issues could not be resolved")
			}
			exit(1)
		}
	},
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
	checkDockerCmd.Flags().BoolVar(&autoFixFlag, "auto-fix", false, "Automatically attempt to fix issues (pull missing images, check disk space)")
	rootCmd.AddCommand(checkDockerCmd)
}
