package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"

	"recac/internal/docker"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/spf13/cobra"
)

// chaosDockerClientFactory allows mocking the Docker client in tests.
var chaosDockerClientFactory = func() (*docker.Client, error) {
	// We pass project name "recac-chaos"
	return docker.NewClient("recac-chaos")
}

var (
	chaosTarget   string
	chaosInterval time.Duration
	chaosDuration time.Duration
	chaosPath     string
	chaosCPU      int
)

var chaosCmd = &cobra.Command{
	Use:   "chaos",
	Short: "Inject controlled failures to test system resilience",
	Long:  `Injects various types of failures (Docker kills, file modifications, CPU stress) into the system to verify resilience.`,
}

var chaosDockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Randomly kill Docker containers",
	Long:  `Continuously finds and kills Docker containers matching a name filter.`,
	RunE:  runChaosDocker,
}

var chaosFileCmd = &cobra.Command{
	Use:   "file",
	Short: "Randomly modify files to trigger watchers",
	Long:  `Randomly touches files in a directory to test hot-reload or file watchers.`,
	RunE:  runChaosFile,
}

var chaosStressCmd = &cobra.Command{
	Use:   "stress",
	Short: "Spike CPU usage",
	Long:  `Spawns goroutines to consume CPU cycles for a specified duration.`,
	RunE:  runChaosStress,
}

func init() {
	rootCmd.AddCommand(chaosCmd)
	chaosCmd.AddCommand(chaosDockerCmd)
	chaosCmd.AddCommand(chaosFileCmd)
	chaosCmd.AddCommand(chaosStressCmd)

	// Flags for Docker
	chaosDockerCmd.Flags().StringVar(&chaosTarget, "target", "", "Name filter for containers (e.g. 'recac-agent')")
	chaosDockerCmd.Flags().DurationVar(&chaosInterval, "interval", 30*time.Second, "Interval between kills")
	chaosDockerCmd.Flags().DurationVar(&chaosDuration, "duration", 5*time.Minute, "Total duration of chaos")
	chaosDockerCmd.MarkFlagRequired("target")

	// Flags for File
	chaosFileCmd.Flags().StringVar(&chaosPath, "path", ".", "Directory to target")
	chaosFileCmd.Flags().DurationVar(&chaosInterval, "interval", 10*time.Second, "Interval between modifications")
	chaosFileCmd.Flags().DurationVar(&chaosDuration, "duration", 1*time.Minute, "Total duration")

	// Flags for Stress
	chaosStressCmd.Flags().IntVar(&chaosCPU, "cpu", 0, "Number of cores to stress (default: all)")
	chaosStressCmd.Flags().DurationVar(&chaosDuration, "duration", 1*time.Minute, "Duration of stress")
}

func runChaosDocker(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		fmt.Println("DEBUG: ctx is nil, using background")
		ctx = context.Background()
	}
	cli, err := chaosDockerClientFactory()
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	fmt.Fprintf(cmd.OutOrStdout(), "🔥 Starting Docker Chaos on '%s' every %s for %s\n", chaosTarget, chaosInterval, chaosDuration)

	timer := time.NewTimer(chaosDuration)
	ticker := time.NewTicker(chaosInterval)
	defer ticker.Stop()

	killRoutine := func() {
		// List containers
		opts := container.ListOptions{
			Filters: filters.NewArgs(filters.Arg("name", chaosTarget)),
		}
		if cli == nil {
			fmt.Println("CRITICAL: cli is nil")
			return
		}
		containers, err := cli.ListContainers(ctx, opts)
		if err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Error listing containers: %v\n", err)
			return
		}

		if len(containers) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No matching containers found.")
			return
		}

		// Pick random
		target := containers[rand.Intn(len(containers))]
		// Check for nil slices to avoid panic
		name := "unknown"
		if len(target.Names) > 0 {
			name = target.Names[0]
		}

		id := "unknown"
		if len(target.ID) >= 12 {
			id = target.ID[:12]
		} else {
			id = target.ID
		}

		fmt.Fprintf(cmd.OutOrStdout(), "💀 Killing container %s (%s)...\n", name, id)

		// Kill (using Remove with force, or Stop? Remove force is effectively kill)
		if err := cli.RemoveContainer(ctx, target.ID, true); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to kill container: %v\n", err)
		}
	}

	// Run immediately once
	killRoutine()

	for {
		select {
		case <-timer.C:
			fmt.Fprintln(cmd.OutOrStdout(), "🛑 Chaos duration finished.")
			return nil
		case <-ticker.C:
			killRoutine()
		case <-ctx.Done():
			return nil
		}
	}
}

func runChaosFile(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "🌪️ Starting File Chaos in '%s' every %s for %s\n", chaosPath, chaosInterval, chaosDuration)

	timer := time.NewTimer(chaosDuration)
	ticker := time.NewTicker(chaosInterval)
	defer ticker.Stop()

	// Gather files
	var files []string
	err := filepath.Walk(chaosPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to scan directory: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found in %s", chaosPath)
	}

	modifyRoutine := func() {
		target := files[rand.Intn(len(files))]
		fmt.Fprintf(cmd.OutOrStdout(), "✍️ Touching file %s...\n", target)

		// Just update mtime
		now := time.Now()
		if err := os.Chtimes(target, now, now); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Failed to touch file: %v\n", err)
		}
	}

	modifyRoutine()

	for {
		select {
		case <-timer.C:
			fmt.Fprintln(cmd.OutOrStdout(), "🛑 Chaos finished.")
			return nil
		case <-ticker.C:
			modifyRoutine()
		}
	}
}

func runChaosStress(cmd *cobra.Command, args []string) error {
	cores := chaosCPU
	if cores <= 0 {
		cores = runtime.NumCPU()
	}

	fmt.Fprintf(cmd.OutOrStdout(), "⚡ Starting CPU Stress on %d cores for %s\n", cores, chaosDuration)

	done := make(chan struct{})
	var wg sync.WaitGroup

	for i := 0; i < cores; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					// Busy loop
					_ = 0 * 0
				}
			}
		}()
	}

	time.Sleep(chaosDuration)
	close(done)
	wg.Wait()
	fmt.Fprintln(cmd.OutOrStdout(), "🛑 Stress finished.")
	return nil
}
