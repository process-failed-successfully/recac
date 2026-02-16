package main

import (
	"context"
	"fmt"
	"time"

	"recac/internal/docker"
	"recac/internal/orchestrator"
	"recac/internal/telemetry"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Cleanup stale agents/containers",
	Long:  `Remove containers or jobs created by the orchestrator that are older than the specified duration.`,
	RunE:  runCleanup,
}

func init() {
	rootCmd.AddCommand(cleanupCmd)

	cleanupCmd.Flags().Duration("older-than", 24*time.Hour, "Remove containers older than this duration")

	viper.BindPFlag("cleanup.older_than", cleanupCmd.Flags().Lookup("older-than"))
}

func runCleanup(cmd *cobra.Command, args []string) error {
	logger := telemetry.NewLogger(viper.GetBool("verbose"), "orchestrator-cleanup", false)

	// Use the mode from root command/config
	mode := viper.GetString("orchestrator.mode")
	olderThan := viper.GetDuration("cleanup.older_than")

	logger.Info("Starting cleanup", "mode", mode, "older_than", olderThan)

	ctx := context.Background()

	if mode == "local" || mode == "docker" {
		projectName := "recac-orchestrator" // Must match what is used in root.go

		dockerCli, err := docker.NewClient(projectName)
		if err != nil {
			return fmt.Errorf("failed to initialize Docker client: %w", err)
		}
		defer dockerCli.Close()

		cleaner := orchestrator.NewCleaner(logger, dockerCli, projectName)
		removed, err := cleaner.Cleanup(ctx, olderThan)
		if err != nil {
			return fmt.Errorf("cleanup failed: %w", err)
		}

		logger.Info("Cleanup finished", "removed_count", len(removed))
		for _, id := range removed {
			if len(id) > 12 {
				fmt.Printf("Removed: %s\n", id[:12])
			} else {
				fmt.Printf("Removed: %s\n", id)
			}
		}

	} else if mode == "k8s" || mode == "kubernetes" {
		logger.Warn("Kubernetes cleanup is not yet implemented")
		// TODO: Implement K8s cleanup (delete old Jobs)
		return nil
	} else {
		return fmt.Errorf("unknown mode: %s", mode)
	}

	return nil
}
