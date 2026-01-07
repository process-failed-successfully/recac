package ui

import (
	"context"
	"fmt"
	"recac/internal/docker"
	"recac/internal/runner"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// GetStatus generates a formatted string with the current status of RECAC.
func GetStatus() string {
	var b strings.Builder

	b.WriteString("--- RECAC Status ---\n")

	// --- Sessions ---
	b.WriteString("\n[Sessions]\n")
	sm, err := runner.NewSessionManager()
	if err != nil {
		b.WriteString(fmt.Sprintf("  Error: failed to initialize session manager: %v\n", err))
	} else {
		sessions, err := sm.ListSessions()
		if err != nil {
			b.WriteString(fmt.Sprintf("  Error: failed to list sessions: %v\n", err))
		} else if len(sessions) == 0 {
			b.WriteString("  No active or past sessions found.\n")
		} else {
			for _, s := range sessions {
				status := strings.ToUpper(s.Status)
				duration := time.Since(s.StartTime).Round(time.Second)
				b.WriteString(fmt.Sprintf("  - %s (PID: %d, Status: %s, Uptime: %s)\n", s.Name, s.PID, status, duration))
				b.WriteString(fmt.Sprintf("    Log: %s\n", s.LogFile))
			}
		}
	}

	// --- Docker ---
	b.WriteString("\n[Docker Environment]\n")
	dockerCli, err := docker.NewClient("") // Project name isn't needed for version check
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

	return b.String()
}
