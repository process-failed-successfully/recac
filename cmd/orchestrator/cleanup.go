package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"recac/internal/docker"
	"recac/internal/orchestrator"
	"recac/internal/telemetry"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cleanupOlderThan time.Duration
	cleanupDryRun    bool
)

var cleanupCmd = &cobra.Command{
	Use:   "cleanup",
	Short: "Cleanup old agent containers",
	Long:  `Remove containers created by the orchestrator (labeled created-by=recac-orchestrator) that are older than the specified duration.`,
	Run: func(cmd *cobra.Command, args []string) {
		runCleanup()
	},
}

func init() {
	rootCmd.AddCommand(cleanupCmd)

	cleanupCmd.Flags().DurationVar(&cleanupOlderThan, "older-than", 24*time.Hour, "Remove containers older than this duration (e.g. 24h, 30m)")
	cleanupCmd.Flags().BoolVar(&cleanupDryRun, "dry-run", false, "Print what would be removed without actually removing it")
}

func runCleanup() {
	// Logger
	logger := telemetry.NewLogger(viper.GetBool("verbose"), "orchestrator", false)
	telemetry.InitLogger(viper.GetBool("verbose"), "orchestrator", false)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("Initializing Docker client for cleanup...")
	dockerCli, err := docker.NewClient("recac-orchestrator")
	if err != nil {
		logger.Error("Failed to initialize Docker client", "error", err)
		os.Exit(1)
	}
	defer dockerCli.Close()

	if err := orchestrator.CleanupContainers(ctx, dockerCli, cleanupOlderThan, cleanupDryRun, logger); err != nil {
		logger.Error("Cleanup failed", "error", err)
		os.Exit(1)
	}

	logger.Info("Cleanup finished successfully")
}
