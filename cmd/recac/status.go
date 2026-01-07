package main

import (
	"context"
	"fmt"
	"os"
	"recac/internal/docker"
	"recac/internal/runner"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func init() {
	rootCmd.AddCommand(statusCmd)
}

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show the status of RECAC sessions and environment",
	Long:  `Displays a summary of all running and completed RECAC sessions, checks the Docker environment, and shows key configuration values.`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := showStatus(); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}
	},
}

func showStatus() error {
	fmt.Println("--- RECAC Status ---")

	// --- Sessions ---
	fmt.Println("\n[Sessions]")
	sm, err := runner.NewSessionManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No active or past sessions found.")
	} else {
		for _, s := range sessions {
			status := strings.ToUpper(s.Status)
			duration := time.Since(s.StartTime).Round(time.Second)
			fmt.Printf("  - %s (PID: %d, Status: %s, Uptime: %s)\n", s.Name, s.PID, status, duration)
			fmt.Printf("    Log: %s\n", s.LogFile)
		}
	}

	// --- Docker ---
	fmt.Println("\n[Docker Environment]")
	dockerCli, err := docker.NewClient("") // Project name isn't needed for version check
	if err != nil {
		fmt.Println("  Docker client failed to initialize. Is Docker running?")
		fmt.Printf("  Error: %v\n", err)
	} else {
		version, err := dockerCli.ServerVersion(context.Background())
		if err != nil {
			fmt.Println("  Could not connect to Docker daemon. Is it running?")
			fmt.Printf("  Error: %v\n", err)
		} else {
			fmt.Printf("  - Docker Version: %s\n", version.Version)
			fmt.Printf("  - API Version: %s\n", version.APIVersion)
			fmt.Printf("  - OS/Arch: %s/%s\n", version.Os, version.Arch)
		}
	}

	// --- Configuration ---
	fmt.Println("\n[Configuration]")
	fmt.Printf("  - Provider: %s\n", viper.GetString("provider"))
	fmt.Printf("  - Model: %s\n", viper.GetString("model"))
	if viper.IsSet("config") {
		fmt.Printf("  - Config File: %s\n", viper.GetString("config"))
	} else {
		fmt.Printf("  - Config File: %s\n", viper.ConfigFileUsed())
	}

	return nil
}
