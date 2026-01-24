package main

import (
	"context"
	"fmt"
	"io/fs"
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"recac/internal/docker"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/spf13/cobra"
)

var (
	chaosPattern  string
	chaosInterval time.Duration
	chaosCount    int
	chaosPath     string
	chaosRate     float64
	chaosCPU      int
	chaosMemory   int64
	chaosDuration time.Duration
	chaosDryRun   bool
)

// Define interface for mocking
type chaosDockerClient interface {
	ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	KillContainer(ctx context.Context, containerID, signal string) error
	Close() error
}

// Factory for dependency injection
var chaosDockerClientFactory = func(project string) (chaosDockerClient, error) {
	return docker.NewClient(project)
}

var chaosCmd = &cobra.Command{
	Use:   "chaos",
	Short: "Inject failures for resilience testing",
	Long:  `Inject various types of failures (Docker, File, Stress) to test system resilience.`,
}

var chaosDockerCmd = &cobra.Command{
	Use:   "docker",
	Short: "Kill random Docker containers",
	RunE:  runChaosDocker,
}

var chaosFileCmd = &cobra.Command{
	Use:   "file",
	Short: "Randomly modify files in a directory",
	RunE:  runChaosFile,
}

var chaosStressCmd = &cobra.Command{
	Use:   "stress",
	Short: "Consume CPU and Memory resources",
	RunE:  runChaosStress,
}

func init() {
	// Docker flags
	chaosDockerCmd.Flags().StringVar(&chaosPattern, "pattern", "", "Name pattern to match containers (required)")
	chaosDockerCmd.Flags().DurationVar(&chaosInterval, "interval", 5*time.Second, "Interval between kills")
	chaosDockerCmd.Flags().IntVar(&chaosCount, "count", 1, "Number of containers to kill per interval")
	chaosDockerCmd.Flags().BoolVar(&chaosDryRun, "dry-run", false, "Print what would be killed without doing it")
	chaosCmd.AddCommand(chaosDockerCmd)

	// File flags
	chaosFileCmd.Flags().StringVar(&chaosPath, "path", "", "Target directory path (required)")
	chaosFileCmd.Flags().Float64Var(&chaosRate, "rate", 0.01, "Modification rate (0.0-1.0)")
	chaosFileCmd.Flags().BoolVar(&chaosDryRun, "dry-run", false, "Print what would be modified without doing it")
	chaosCmd.AddCommand(chaosFileCmd)

	// Stress flags
	chaosStressCmd.Flags().IntVar(&chaosCPU, "cpu", 1, "Number of CPU workers")
	chaosStressCmd.Flags().Int64Var(&chaosMemory, "memory", 100, "Memory to allocate in MB")
	chaosStressCmd.Flags().DurationVar(&chaosDuration, "duration", 30*time.Second, "Duration of stress test")
	chaosCmd.AddCommand(chaosStressCmd)

	rootCmd.AddCommand(chaosCmd)
}

func runChaosDocker(cmd *cobra.Command, args []string) error {
	if chaosPattern == "" {
		return fmt.Errorf("pattern is required")
	}

	ctx := context.Background()
	cli, err := chaosDockerClientFactory("chaos-docker")
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer cli.Close()

	fmt.Fprintf(cmd.OutOrStdout(), "Starting Chaos Docker (Pattern: %s, Interval: %s, Count: %d)\n", chaosPattern, chaosInterval, chaosCount)

	ticker := time.NewTicker(chaosInterval)
	defer ticker.Stop()

	// Initial run
	if err := killRandomContainers(ctx, cmd, cli); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
	}

	// Loop until interrupted (ctrl-c handled by caller usually, but here we run once? No, chaos usually runs for a bit.
	// But CLI commands typically block. Let's make it run forever until interrupt or maybe add a duration flag?
	// For simplicity, let's run ONCE if interval is 0, or loop.
	// Actually, let's respect the ticker. But user needs to stop it.
	// Since we don't have a duration flag for docker command, let's just loop forever.
	// But wait, testing this loop is hard.
	// Let's check context cancellation.

	// For now, let's just run it loop until context is done (which Cobra doesn't set by default on ctrl-c unless configured).
	// But standard CLI behavior for `watch`-like commands is to block.
	// However, to make it testable and usable, maybe just run ONCE by default unless --watch or similar?
	// The prompt said "Interval between kills", implying a loop.

	// Let's add a timeout/duration flag or just loop.
	// To keep it simple and testable, I'll loop but check for context cancellation.
	// I'll also add a limit to iterations for testing? No.

	// I'll make it loop. Users use Ctrl-C.

	for {
		select {
		case <-ticker.C:
			if err := killRandomContainers(ctx, cmd, cli); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Error: %v\n", err)
			}
		case <-cmd.Context().Done():
			return nil
		}
	}
}

func killRandomContainers(ctx context.Context, cmd *cobra.Command, cli chaosDockerClient) error {
	containers, err := cli.ListContainers(ctx, container.ListOptions{})
	if err != nil {
		return err
	}

	var candidates []types.Container
	for _, c := range containers {
		for _, name := range c.Names {
			// Name usually starts with /, remove it for matching
			cleanName := strings.TrimPrefix(name, "/")
			match, _ := filepath.Match(chaosPattern, cleanName)
			if match || strings.Contains(cleanName, chaosPattern) {
				candidates = append(candidates, c)
				break
			}
		}
	}

	if len(candidates) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "No matching containers found.")
		return nil
	}

	// Shuffle
	rand.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	toKill := chaosCount
	if toKill > len(candidates) {
		toKill = len(candidates)
	}

	for i := 0; i < toKill; i++ {
		c := candidates[i]
		name := strings.TrimPrefix(c.Names[0], "/")
		shortID := c.ID
		if len(shortID) > 12 {
			shortID = shortID[:12]
		}
		if chaosDryRun {
			fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would kill container %s (%s)\n", name, shortID)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "💀 Killing container %s (%s)...\n", name, shortID)
			if err := cli.KillContainer(ctx, c.ID, "KILL"); err != nil {
				return fmt.Errorf("failed to kill %s: %w", name, err)
			}
		}
	}
	return nil
}

func runChaosFile(cmd *cobra.Command, args []string) error {
	if chaosPath == "" {
		return fmt.Errorf("path is required")
	}

	fmt.Fprintf(cmd.OutOrStdout(), "Starting Chaos File (Path: %s, Rate: %.2f)\n", chaosPath, chaosRate)

	return filepath.WalkDir(chaosPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Random chance
		if rand.Float64() < chaosRate {
			if chaosDryRun {
				fmt.Fprintf(cmd.OutOrStdout(), "[DRY RUN] Would corrupt %s\n", path)
				return nil
			}

			fmt.Fprintf(cmd.OutOrStdout(), "💥 Corrupting %s\n", path)
			f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to open %s: %v\n", path, err)
				return nil // Continue walking
			}
			defer f.Close()

			if _, err := f.WriteString("\n# CHAOS WAS HERE\n"); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "Failed to write to %s: %v\n", path, err)
			}
		}
		return nil
	})
}

func runChaosStress(cmd *cobra.Command, args []string) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Starting Chaos Stress (CPU: %d, Mem: %dMB, Duration: %s)\n", chaosCPU, chaosMemory, chaosDuration)

	done := make(chan struct{})
	time.AfterFunc(chaosDuration, func() {
		close(done)
	})

	// Memory stress
	if chaosMemory > 0 {
		go func() {
			blockSize := 1024 * 1024 // 1MB
			blocks := make([][]byte, 0)
			for i := 0; i < int(chaosMemory); i++ {
				select {
				case <-done:
					return
				default:
					blocks = append(blocks, make([]byte, blockSize))
					// Touch memory to ensure allocation
					for j := 0; j < blockSize; j += 4096 {
						blocks[i][j] = 1
					}
					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}

	// CPU stress
	for i := 0; i < chaosCPU; i++ {
		go func() {
			for {
				select {
				case <-done:
					return
				default:
					// Busy loop
					for j := 0; j < 1000000; j++ {
						_ = j * j
					}
					runtime.Gosched()
				}
			}
		}()
	}

	<-done
	fmt.Fprintln(cmd.OutOrStdout(), "Chaos Stress Finished.")
	return nil
}
