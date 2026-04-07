package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/lipgloss"
)

func listBlockers(host, jobID string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/%s/blockers", host, jobID))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch blockers: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if len(jobs) == 0 {
		fmt.Fprintf(stdout, "No blockers found for job %s.\n", jobID)
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
		Foreground(lipgloss.Color("252")).
		Padding(0, 1)

	rowStyle := lipgloss.NewStyle().
		Padding(0, 1)

	// Define column widths explicitly using lipgloss Width
	idCol := lipgloss.NewStyle().Width(15)
	summaryCol := lipgloss.NewStyle().Width(40)
	statusCol := lipgloss.NewStyle().Width(25)
	durationCol := lipgloss.NewStyle().Width(20)

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Blockers of %s (%d)", jobID, len(jobs))))
	fmt.Fprintln(stdout, "")

	// Table Header (using lipgloss width instead of fmt %-15s, since fmt counts ANSI codes)
	fmt.Fprintf(stdout, "%s %s %s %s\n",
		idCol.Render(headerStyle.Render("ID")),
		summaryCol.Render(headerStyle.Render("Summary")),
		statusCol.Render(headerStyle.Render("Status")),
		durationCol.Render(headerStyle.Render("Duration")),
	)

	for _, job := range jobs {
		duration := "N/A"
		if !job.StartTime.IsZero() {
			endTime := job.EndTime
			if endTime.IsZero() {
				endTime = time.Now()
			}
			duration = endTime.Sub(job.StartTime).Round(time.Second).String()
		}

		statusDisplay := job.Status
		if job.Progress != nil {
			statusDisplay = fmt.Sprintf("%s (%d%%)", job.Status, *job.Progress)
		}
		if job.StatusMessage != nil {
			statusDisplay = fmt.Sprintf("%s - %s", statusDisplay, *job.StatusMessage)
		}
		statusDisplay = limitString(statusDisplay, 25)

		fmt.Fprintf(stdout, "%s %s %s %s\n",
			idCol.Render(rowStyle.Render(job.ID)),
			summaryCol.Render(rowStyle.Render(limitString(job.Summary, 38))),
			statusCol.Render(rowStyle.Render(statusDisplay)),
			durationCol.Render(rowStyle.Render(duration)),
		)
	}
}
