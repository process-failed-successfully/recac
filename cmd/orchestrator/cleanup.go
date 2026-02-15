package main

import (
	"context"
	"fmt"
	"recac/internal/docker"
	"recac/internal/orchestrator"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/spf13/cobra"
)

var (
	olderThanDuration time.Duration
	forceCleanup      bool
)

func init() {
	cleanupCmd.Flags().DurationVar(&olderThanDuration, "older-than", 24*time.Hour, "Cleanup containers older than this duration")
	cleanupCmd.Flags().BoolVar(&forceCleanup, "force", false, "Force removal of running containers")
	rootCmd.AddCommand(cleanupCmd)
}

// CleanerDockerClient defines the subset of DockerClient needed for cleanup.
// It matches orchestrator.DockerClient but adds Close for the concrete type.
type CleanerDockerClient interface {
	orchestrator.DockerClient
	Close() error
}

var dockerClientFactory = func(project string) (CleanerDockerClient, error) {
	return docker.NewClient(project)
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Cleanup old agent containers",
	Long:  `Removes Docker containers created by the orchestrator that exceed the specified age.`,
	RunE:  runCleanup,
}

func runCleanup(cmd *cobra.Command, args []string) error {
	ctx := context.Background()
	out := cmd.OutOrStdout()

	// We need a Docker client. Re-using internal/docker/client logic.
	// Use a dummy project name for client init.
	client, err := dockerClientFactory("cleanup-tool")
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}
	defer client.Close()

	fmt.Fprintf(out, "Cleaning up containers created by recac-orchestrator older than %s...\n", olderThanDuration)

	// Calculate cutoff time
	cutoff := time.Now().Add(-olderThanDuration)

	// List containers with label
	// Since our internal client wraps API, we use ListContainers which exposes types.Container
	// But we need to filter. The internal ListContainers takes container.ListOptions.

	filterArgs := filters.NewArgs()
	filterArgs.Add("label", "created-by=recac-orchestrator")

	containers, err := client.ListContainers(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	if len(containers) == 0 {
		fmt.Fprintln(out, "No matching containers found.")
		return nil
	}

	fmt.Fprintf(out, "Found %d matching containers. Checking age...\n", len(containers))

	count := 0
	for _, c := range containers {
		// Check creation time
		created := time.Unix(c.Created, 0)
		if created.Before(cutoff) {
			fmt.Fprintf(out, "removing %s (ID: %s, Created: %s)...\n", c.Names[0], c.ID[:12], created.Format(time.RFC3339))

			// Stop first if running
			if c.State == "running" {
				if err := client.StopContainer(ctx, c.ID); err != nil {
					fmt.Fprintf(out, "  Warning: failed to stop %s: %v\n", c.ID[:12], err)
					if !forceCleanup {
						continue
					}
				}
			}

			// Remove
			if err := client.RemoveContainer(ctx, c.ID, forceCleanup); err != nil {
				fmt.Fprintf(out, "  Error removing %s: %v\n", c.ID[:12], err)
			} else {
				count++
			}
		}
	}

	fmt.Fprintf(out, "Cleanup complete. Removed %d containers.\n", count)
	return nil
}
