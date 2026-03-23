package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"recac/internal/orchestrator"
	"github.com/charmbracelet/lipgloss"
)

func sanitizeEnvVarName(id string) string {
	var sb strings.Builder
	sb.Grow(len(id))
	for i := 0; i < len(id); i++ {
		c := id[i]
		if c >= 'a' && c <= 'z' {
			sb.WriteByte(c - 32)
		} else if (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			sb.WriteByte(c)
		} else if c == '-' {
			sb.WriteByte('_')
		} else {
			sb.WriteByte('_')
		}
	}
	return sb.String()
}

func inspectDataflow(host, jobID string) {
	// 1. Fetch Target Job
	targetJobResp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer targetJobResp.Body.Close()

	if targetJobResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(targetJobResp.Body)
		fmt.Fprintf(stdout, "Failed to fetch target job details: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var targetJob orchestrator.JobInfo
	if err := json.NewDecoder(targetJobResp.Body).Decode(&targetJob); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if len(targetJob.WorkItem.DependsOn) == 0 {
		fmt.Fprintf(stdout, "Job %s has no dependencies.\n", jobID)
		return
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	jobStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205"))

	varStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("43"))

	valStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Dataflow Inspection: %s", jobID)))
	fmt.Fprintln(stdout, "")

	hasDataflow := false

	// 2. Fetch Dependencies and process outputs
	for _, depID := range targetJob.WorkItem.DependsOn {
		depResp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, depID))
		if err != nil || depResp.StatusCode != http.StatusOK {
			if depResp != nil {
				depResp.Body.Close()
			}
			fmt.Fprintf(stdout, "Dependency %s:\n  Could not fetch details.\n\n", jobStyle.Render(depID))
			continue
		}

		var depJob orchestrator.JobInfo
		if err := json.NewDecoder(depResp.Body).Decode(&depJob); err != nil {
			depResp.Body.Close()
			continue
		}
		depResp.Body.Close()

		if len(depJob.Outputs) == 0 {
			fmt.Fprintf(stdout, "Dependency %s:\n  No outputs generated.\n\n", jobStyle.Render(depID))
			continue
		}

		hasDataflow = true
		fmt.Fprintf(stdout, "%s:\n", headerStyle.Render(fmt.Sprintf("From %s", depID)))

		prefix := fmt.Sprintf("DEP_%s_", sanitizeEnvVarName(depID))
		for k, v := range depJob.Outputs {
			envVarName := prefix + strings.ToUpper(k)
			fmt.Fprintf(stdout, "  %s=%s\n", varStyle.Render(envVarName), valStyle.Render(v))
		}
		fmt.Fprintln(stdout, "")
	}

	if !hasDataflow {
		fmt.Fprintf(stdout, "No dataflow variables injected from dependencies.\n")
	}
}
