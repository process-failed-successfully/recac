package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"recac/internal/docker"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/spf13/cobra"
)

var (
	cleanupOlderThan time.Duration
	cleanupDryRun    bool
)

// CleanerDockerClient defines the interface needed for cleanup
type CleanerDockerClient interface {
	ListContainers(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	RemoveContainer(ctx context.Context, containerID string, force bool) error
	Close() error
}

var cleanerDockerFactory = func() (CleanerDockerClient, error) {
	return docker.NewClient("recac-cleanup")
}

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Cleanup stale agent containers",
	Long: `Removes Docker containers created by the orchestrator that are older than the specified duration.
This is useful for cleaning up stuck or abandoned agent containers.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx := context.Background()

		// Initialize Docker Client
		cli, err := cleanerDockerFactory()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to initialize Docker client: %v\n", err)
			os.Exit(1)
		}
		defer cli.Close()

		// Filter for containers created by orchestrator
		filterArgs := filters.NewArgs()
		filterArgs.Add("label", "created-by=recac-orchestrator")

		containers, err := cli.ListContainers(ctx, container.ListOptions{
			All:     true,
			Filters: filterArgs,
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to list containers: %v\n", err)
			os.Exit(1)
		}

		cutoff := time.Now().Add(-cleanupOlderThan)
		count := 0
		removed := 0

		fmt.Printf("Scanning for containers older than %s (cutoff: %s)...\n", cleanupOlderThan, cutoff.Format(time.RFC3339))

		for _, c := range containers {
			created := time.Unix(c.Created, 0)
			if created.Before(cutoff) {
				count++
				workItem := c.Labels["work-item"]
				fmt.Printf("Found stale container: %s (ID: %s, WorkItem: %s, Created: %s)\n",
					c.Names[0], c.ID[:12], workItem, created.Format(time.RFC3339))

				if !cleanupDryRun {
					fmt.Printf("Removing container %s... ", c.ID[:12])
					if err := cli.RemoveContainer(ctx, c.ID, true); err != nil {
						fmt.Printf("Failed: %v\n", err)
					} else {
						fmt.Printf("Success\n")
						removed++
					}
				}
			}
		}

		if cleanupDryRun {
			fmt.Printf("\n[Dry Run] Found %d stale containers. Run without --dry-run to remove them.\n", count)
		} else {
			fmt.Printf("\nCleanup complete. Removed %d/%d stale containers.\n", removed, count)
		}
	},
}

func init() {
	cleanupCmd.Flags().DurationVar(&cleanupOlderThan, "older-than", 24*time.Hour, "Remove containers older than this duration")
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "List containers without removing them")

	rootCmd.AddCommand(cleanupCmd)
}
