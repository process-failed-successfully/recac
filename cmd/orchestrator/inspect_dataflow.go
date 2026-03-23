package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

// sanitizeEnvVarName is a helper to sanitize job ID for env var name
// Mirrors the one in orchestrator package.
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
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch job details: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Pretty print
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	jobStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Bold(true)

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Dataflow Inspection: %s", job.ID)))
	fmt.Fprintln(stdout, "")

	if len(job.WorkItem.DependsOn) == 0 {
		fmt.Fprintln(stdout, "This job has no upstream dependencies.")
		return
	}

	fmt.Fprintf(stdout, "Upstream Dependencies: %s\n\n", strings.Join(job.WorkItem.DependsOn, ", "))

	foundAny := false

	// Check each upstream job's outputs
	for _, depID := range job.WorkItem.DependsOn {
		depResp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, depID))
		if err != nil {
			fmt.Fprintf(stdout, "%s:\n  Could not fetch dependency: %v\n\n", jobStyle.Render(depID), err)
			continue
		}

		if depResp.StatusCode != http.StatusOK {
			depResp.Body.Close()
			fmt.Fprintf(stdout, "%s:\n  Could not fetch dependency (status %d)\n\n", jobStyle.Render(depID), depResp.StatusCode)
			continue
		}

		var depJob orchestrator.JobInfo
		if err := json.NewDecoder(depResp.Body).Decode(&depJob); err != nil {
			depResp.Body.Close()
			fmt.Fprintf(stdout, "%s:\n  Failed to decode dependency\n\n", jobStyle.Render(depID))
			continue
		}
		depResp.Body.Close()

		if len(depJob.Outputs) == 0 {
			fmt.Fprintf(stdout, "%s:\n  (No outputs generated)\n\n", jobStyle.Render(depID))
			continue
		}

		foundAny = true
		fmt.Fprintf(stdout, "%s:\n", jobStyle.Render(depID))

		prefix := fmt.Sprintf("DEP_%s_", sanitizeEnvVarName(depID))

		for k, v := range depJob.Outputs {
			envName := prefix + strings.ToUpper(k)
			fmt.Fprintf(stdout, "  %s  ->  %s=%s\n", headerStyle.Render("Output '"+k+"'"), headerStyle.Render(envName), limitString(v, 60))
		}
		fmt.Fprintln(stdout, "")
	}

	if foundAny {
		fmt.Fprintln(stdout, "These variables will be injected into the job's environment when it runs.")
	} else {
		fmt.Fprintln(stdout, "None of the upstream dependencies have generated outputs yet.")
	}
}
