package ui

import (
	"bytes"
	"context"
	"fmt"
	"recac/internal/docker"
	"recac/internal/runner"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// GetStatus gathers and formats the status of RECAC sessions, environment, and configuration.
func GetStatus() string {
	var b bytes.Buffer

	b.WriteString("--- RECAC Status ---\n")

	// --- Sessions ---
	b.WriteString("\n[Sessions]\n")
	sm, err := runner.NewSessionManager()
	if err != nil {
		fmt.Fprintf(&b, "Error: failed to initialize session manager: %v\n", err)
	} else {
		sessions, err := sm.ListSessions()
		if err != nil {
			fmt.Fprintf(&b, "Error: failed to list sessions: %v\n", err)
		} else if len(sessions) == 0 {
			b.WriteString("No active or past sessions found.\n")
		} else {
			for _, s := range sessions {
				status := strings.ToUpper(s.Status)
				duration := time.Since(s.StartTime).Round(time.Second)
				fmt.Fprintf(&b, "  - %s (PID: %d, Status: %s, Uptime: %s)\n", s.Name, s.PID, status, duration)
				fmt.Fprintf(&b, "    Log: %s\n", s.LogFile)
			}
		}
	}

	// --- Docker ---
	b.WriteString("\n[Docker Environment]\n")
	dockerCli, err := docker.NewClient("") // Project name isn't needed for version check
	if err != nil {
		fmt.Fprintf(&b, "  Docker client failed to initialize. Is Docker running?\n")
		fmt.Fprintf(&b, "  Error: %v\n", err)
	} else {
		version, err := dockerCli.ServerVersion(context.Background())
		if err != nil {
			fmt.Fprintf(&b, "  Could not connect to Docker daemon. Is it running?\n")
			fmt.Fprintf(&b, "  Error: %v\n", err)
		} else {
			fmt.Fprintf(&b, "  - Docker Version: %s\n", version.Version)
			fmt.Fprintf(&b, "  - API Version: %s\n", version.APIVersion)
			fmt.Fprintf(&b, "  - OS/Arch: %s/%s\n", version.Os, version.Arch)
		}
	}

	// --- Configuration ---
	b.WriteString("\n[Configuration]\n")
	fmt.Fprintf(&b, "  - Provider: %s\n", viper.GetString("provider"))
	fmt.Fprintf(&b, "  - Model: %s\n", viper.GetString("model"))
	if viper.IsSet("config") {
		fmt.Fprintf(&b, "  - Config File: %s\n", viper.GetString("config"))
	} else {
		fmt.Fprintf(&b, "  - Config File: %s\n", viper.ConfigFileUsed())
	}

	return b.String()
}
