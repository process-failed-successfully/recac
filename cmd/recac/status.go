package main

import (
	"context"
	"fmt"
	"os"

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
	sm, err := newSessionManager()
	if err != nil {
		return fmt.Errorf("failed to initialize session manager: %w", err)
	}

	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
	} else {
		fmt.Printf("%-20s %-10s %-10s %-20s %s\n", "NAME", "STATUS", "PID", "STARTED", "WORKSPACE")
		fmt.Println("--------------------------------------------------------------------------------")
		for _, session := range sessions {
			started := session.StartTime.Format("2006-01-02 15:04:05")
			fmt.Printf("%-20s %-10s %-10d %-20s %s\n",
				session.Name,
				session.Status,
				session.PID,
				started,
				session.Workspace,
			)
		}
	}

	// --- Docker ---
	fmt.Println("\n[Docker Environment]")
	dockerCli, err := newDockerClient("") // Project name isn't needed for version check
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
