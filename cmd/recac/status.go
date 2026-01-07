package main

import (
	"context"
	"fmt"
	"io"
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
		// Dependencies are created here and passed to the logic function
		sm, err := runner.NewSessionManager()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to initialize session manager: %v\n", err)
			exit(1)
		}

		dockerCli, dockerErr := docker.NewClient("")

		if err := showStatus(os.Stdout, sm, dockerCli, dockerErr); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			exit(1)
		}
	},
}

// showStatus contains the core logic for displaying the status, now decoupled from dependency creation.
// This makes it easier to test.
func showStatus(out io.Writer, sm *runner.SessionManager, dockerCli *docker.Client, dockerErr error) error {
	// --- Docker Environment ---
	fmt.Fprintln(out, "[Docker Environment]")
	if dockerErr != nil {
		fmt.Fprintln(out, "  Docker client failed to initialize. Is Docker running?")
		fmt.Fprintf(out, "  Error: %v\n", dockerErr)
	} else {
		version, err := dockerCli.ServerVersion(context.Background())
		if err != nil {
			fmt.Fprintln(out, "  Could not connect to Docker daemon. Is it running?")
			fmt.Fprintf(out, "  Error: %v\n", err)
		} else {
			fmt.Fprintf(out, "  - Docker Version: %s\n", version.Version)
			fmt.Fprintf(out, "  - API Version: %s\n", version.APIVersion)
			fmt.Fprintf(out, "  - OS/Arch: %s/%s\n", version.Os, version.Arch)
		}
	}

	// --- Sessions ---
	fmt.Fprintln(out, "\n[Sessions]")
	sessions, err := sm.ListSessions()
	if err != nil {
		return fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		fmt.Fprintln(out, "  No active or past sessions found.")
	} else {
		// Table Headers
		fmt.Fprintf(out, "  %-25s %-10s %-15s %-15s %-15s\n", "NAME", "PID", "SESSION STATUS", "CONTAINER", "UPTIME")
		fmt.Fprintf(out, "  %s\n", strings.Repeat("-", 80))

		for _, s := range sessions {
			sessionStatus := strings.ToUpper(s.Status)
			uptime := time.Since(s.StartTime).Round(time.Second)

			// Determine Container Status
			var containerStatus string
			if dockerCli != nil {
				containerName := fmt.Sprintf("recac-%s", s.Name)
				state, err := dockerCli.ContainerState(context.Background(), containerName)
				if err != nil {
					containerStatus = "NOT FOUND"
				} else {
					containerStatus = strings.ToUpper(state.Status)
				}
			} else {
				containerStatus = "UNKNOWN" // Docker client not available
			}

			fmt.Fprintf(out, "  %-25s %-10d %-15s %-15s %-15s\n", s.Name, s.PID, sessionStatus, containerStatus, uptime)
		}

		// --- Log Commands ---
		fmt.Fprintln(out, "\n[Logs]")
		fmt.Fprintln(out, "  Use the following commands to view logs for a session:")
		for _, s := range sessions {
			fmt.Fprintf(out, "  - recac logs %s\n", s.Name)
		}
	}

	// --- Configuration ---
	fmt.Fprintln(out, "\n[Configuration]")
	fmt.Fprintf(out, "  - Provider: %s\n", viper.GetString("provider"))
	fmt.Fprintf(out, "  - Model: %s\n", viper.GetString("model"))
	if viper.ConfigFileUsed() != "" {
		fmt.Fprintf(out, "  - Config File: %s\n", viper.ConfigFileUsed())
	} else {
		fmt.Fprintln(out, "  - Config File: Not found")
	}
	fmt.Fprintln(out) // Extra line for spacing

	return nil
}
