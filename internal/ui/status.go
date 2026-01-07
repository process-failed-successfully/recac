package ui

import (
	"context"
	"fmt"
	"recac/internal/docker"
	"recac/internal/runner"
	"strings"

	"github.com/spf13/viper"
)

// NewSessionManager is a variable that holds the function to create a new session manager.
// This can be replaced in tests to inject a mock session manager.
var NewSessionManager = runner.NewSessionManager

// NewDockerClient is a variable that holds the function to create a new Docker client.
// This can be replaced in tests to inject a mock Docker client.
var NewDockerClient = docker.NewClient

// GetStatus generates a detailed status report of the RECAC environment.
// It includes session statuses, Docker environment details, and current configuration.
func GetStatus() (string, error) {
	var b strings.Builder

	b.WriteString("--- RECAC Status ---\n")

	// --- Sessions ---
	b.WriteString("\n[Sessions]\n")
	sm, err := NewSessionManager()
	if err != nil {
		return "", fmt.Errorf("failed to initialize session manager: %w", err)
	}

	sessions, err := sm.ListSessions()
	if err != nil {
		return "", fmt.Errorf("failed to list sessions: %w", err)
	}

	if len(sessions) == 0 {
		b.WriteString("No active or past sessions found.\n")
	} else {
		// Use a table format for sessions
		b.WriteString(fmt.Sprintf("%-20s %-10s %-10s %-20s %s\n", "NAME", "STATUS", "PID", "STARTED", "WORKSPACE"))
		b.WriteString(strings.Repeat("-", 80) + "\n")
		for _, session := range sessions {
			started := session.StartTime.Format("2006-01-02 15:04:05")
			b.WriteString(fmt.Sprintf("%-20s %-10s %-10d %-20s %s\n",
				session.Name,
				session.Status,
				session.PID,
				started,
				session.Workspace,
			))
		}
	}

	// --- Docker ---
	b.WriteString("\n[Docker Environment]\n")
	dockerCli, err := NewDockerClient("") // Project name isn't needed for version check
	if err != nil {
		b.WriteString("  Docker client failed to initialize. Is Docker running?\n")
		b.WriteString(fmt.Sprintf("  Error: %v\n", err))
	} else {
		version, err := dockerCli.ServerVersion(context.Background())
		if err != nil {
			b.WriteString("  Could not connect to Docker daemon. Is it running?\n")
			b.WriteString(fmt.Sprintf("  Error: %v\n", err))
		} else {
			b.WriteString(fmt.Sprintf("  - Docker Version: %s\n", version.Version))
			b.WriteString(fmt.Sprintf("  - API Version: %s\n", version.APIVersion))
			b.WriteString(fmt.Sprintf("  - OS/Arch: %s/%s\n", version.Os, version.Arch))
		}
	}

	// --- Configuration ---
	b.WriteString("\n[Configuration]\n")
	b.WriteString(fmt.Sprintf("  - Provider: %s\n", viper.GetString("provider")))
	b.WriteString(fmt.Sprintf("  - Model: %s\n", viper.GetString("model")))
	if viper.IsSet("config") {
		b.WriteString(fmt.Sprintf("  - Config File: %s\n", viper.GetString("config")))
	} else {
		b.WriteString(fmt.Sprintf("  - Config File: %s\n", viper.ConfigFileUsed()))
	}

	return b.String(), nil
}
