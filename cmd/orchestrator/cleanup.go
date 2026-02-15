package main

import (
	"context"
	"fmt"
	"recac/internal/docker"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/spf13/cobra"
)

var (
	cleanupOlderThan time.Duration
	cleanupForce     bool
	cleanupDryRun    bool
)

func init() {
	cleanupCmd.Flags().DurationVar(&cleanupOlderThan, "older-than", 24*time.Hour, "Cleanup containers older than this duration")
	cleanupCmd.Flags().BoolVar(&cleanupForce, "force", false, "Force removal of containers")
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Print actions without executing")

	rootCmd.AddCommand(cleanupCmd)
}

// Interface to allow mocking
type cleanupDockerClient interface {
	CheckDaemon(ctx context.Context) error
	ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	Close() error
}

var dockerClientFactory = func() (cleanupDockerClient, error) {
	return docker.NewClient("recac-cleanup")
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Clean up leaked agent containers",
	Long:  `Clean up Docker containers created by recac-orchestrator that have been running for too long or are stopped.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx := context.Background()

		cli, err := dockerClientFactory()
		if err != nil {
			return fmt.Errorf("failed to create docker client: %w", err)
		}
		defer cli.Close()

		// Verify daemon
		if err := cli.CheckDaemon(ctx); err != nil {
			return fmt.Errorf("docker daemon not reachable: %w", err)
		}

		fmt.Printf("Cleaning up containers older than %v (DryRun: %v)...\n", cleanupOlderThan, cleanupDryRun)

		// List containers with label "created-by=recac-orchestrator"
		opts := container.ListOptions{
			All: true,
			Filters: filters.NewArgs(
				filters.Arg("label", "created-by=recac-orchestrator"),
			),
		}

		containers, err := cli.ListContainers(ctx, opts)
		if err != nil {
			return fmt.Errorf("failed to list containers: %w", err)
		}

		removedCount := 0
		threshold := time.Now().Add(-cleanupOlderThan)

		for _, c := range containers {
			created := time.Unix(c.Created, 0)

			if created.Before(threshold) {
				name := "unknown"
				if len(c.Names) > 0 {
					name = c.Names[0]
				}
				age := time.Since(created).Round(time.Second)
				fmt.Printf("Found candidate: %s (ID: %s, State: %s, Age: %s)\n", name, c.ID[:12], c.State, age)

				if !cleanupDryRun {
					fmt.Printf("Removing %s...\n", c.ID[:12])
					// If container is running, we might need force if --force is set, or if we want to stop it first.
					// docker remove force=true kills it.
					// We pass cleanupForce as force argument.
					if err := cli.RemoveContainer(ctx, c.ID, cleanupForce); err != nil {
						fmt.Printf("Error removing %s: %v\n", c.ID[:12], err)
					} else {
						removedCount++
						fmt.Printf("Removed %s\n", c.ID[:12])
					}
				} else {
					removedCount++ // Count as would-be removed
				}
			}
		}

		if cleanupDryRun {
			fmt.Printf("Dry run complete. Would remove %d containers.\n", removedCount)
		} else {
			fmt.Printf("Cleanup complete. Removed %d containers.\n", removedCount)
		}

		return nil
	},
}
