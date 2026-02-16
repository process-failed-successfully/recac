package main

import (
	"context"
	"os"
	"time"

	"recac/internal/docker"
	"recac/internal/orchestrator"
	"recac/internal/telemetry"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	cleanupCmd := &cobra.Command{
		Use:   "cleanup",
		Short: "Clean up old containers",
		Long:  `Cleanup containers created by the orchestrator that are older than the specified duration.`,
		Run:   runCleanup,
	}

	cleanupCmd.Flags().Duration("max-age", 24*time.Hour, "Maximum age of containers to keep")
	cleanupCmd.Flags().Bool("dry-run", false, "Dry run mode (do not delete)")
	viper.BindPFlag("cleanup.max_age", cleanupCmd.Flags().Lookup("max-age"))
	viper.BindPFlag("cleanup.dry_run", cleanupCmd.Flags().Lookup("dry-run"))

	rootCmd.AddCommand(cleanupCmd)
}

func runCleanup(cmd *cobra.Command, args []string) {
	logger := telemetry.NewLogger(viper.GetBool("verbose"), "orchestrator-cleanup", false)

	maxAge := viper.GetDuration("cleanup.max_age")
	dryRun := viper.GetBool("cleanup.dry_run")

	projectName := "recac-orchestrator" // Should match run.go
	dockerCli, err := docker.NewClient(projectName)
	if err != nil {
		logger.Error("Failed to initialize Docker client", "error", err)
		os.Exit(1)
	}

	cleaner := orchestrator.NewCleaner(logger, dockerCli)

	ctx := context.Background()
	removed, err := cleaner.Cleanup(ctx, maxAge, dryRun)
	if err != nil {
		logger.Error("Cleanup failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Cleanup completed", "removed_count", len(removed))
}
