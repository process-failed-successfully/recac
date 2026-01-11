package ui

import (
	"bytes"
	"context"
	"fmt"
	"recac/internal/docker"
	"recac/internal/k8s"
	"recac/internal/runner"
	"strings"
	"time"

	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/api/errors"
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

	// --- Kubernetes ---
	b.WriteString("\n[Kubernetes Environment]\n")
	k8sClient, err := k8s.NewClient()
	if err != nil {
		fmt.Fprintf(&b, "  Kubernetes client failed to initialize.\n")
		fmt.Fprintf(&b, "  Error: %v\n", err)
	} else {
		// Display K8s context
		contextName, err := k8sClient.GetCurrentContext()
		if err != nil {
			fmt.Fprintf(&b, "  Could not determine Kubernetes context: %v\n", err)
		} else if contextName == "" {
			b.WriteString("  No active Kubernetes configuration found.\n")
		} else {
			fmt.Fprintf(&b, "  - Context: %s\n", contextName)

			// Check for Orchestrator
			deployment, err := k8sClient.GetOrchestratorDeployment(context.Background())
			if err != nil {
				if errors.IsNotFound(err) {
					fmt.Fprintf(&b, "  - Orchestrator: Not Found\n")
				} else {
					fmt.Fprintf(&b, "  - Orchestrator: Error checking status (%v)\n", err)
				}
			} else if deployment != nil {
				status := "Not Ready"
				if deployment.Status.ReadyReplicas > 0 {
					status = "Ready"
				}
				fmt.Fprintf(&b, "  - Orchestrator: %s (%d/%d replicas ready)\n", status, deployment.Status.ReadyReplicas, deployment.Status.Replicas)
			}

			// List Agent Pods
			pods, err := k8sClient.ListAgentPods(context.Background())
			if err != nil {
				fmt.Fprintf(&b, "  - Agent Pods: Error listing pods (%v)\n", err)
			} else {
				fmt.Fprintf(&b, "  - Agent Pods: Found %d\n", len(pods))
				for _, pod := range pods {
					ticket := pod.Labels["ticket"]
					if ticket == "" {
						ticket = "N/A"
					}
					fmt.Fprintf(&b, "    - %s (Ticket: %s, Status: %s)\n", pod.Name, ticket, pod.Status.Phase)
				}
			}
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
