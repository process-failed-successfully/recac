package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/lipgloss"
)

func printTimeline(host string, limit int) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("state", "all")
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stdout, "Failed to fetch jobs: status %s\n", resp.Status)
		exitFunc(1)
		return
	}

	var allJobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&allJobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Filter out jobs that haven't started
	var jobs []orchestrator.JobInfo
	for _, j := range allJobs {
		if !j.StartTime.IsZero() {
			jobs = append(jobs, j)
		}
	}

	if len(jobs) == 0 {
		fmt.Fprintln(stdout, "No started jobs to display in timeline.")
		return
	}

	// Sort by StartTime ascending so we can easily take the N most recent?
	// Wait, we want the most recent `limit` jobs, so sort descending, slice, then sort ascending for display.
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].StartTime.After(jobs[j].StartTime)
	})

	if limit > 0 && len(jobs) > limit {
		jobs = jobs[:limit]
	}

	// Sort back to ascending for top-to-bottom chronological display
	sort.SliceStable(jobs, func(i, j int) bool {
		return jobs[i].StartTime.Before(jobs[j].StartTime)
	})

	// Find global min start and max end
	minStart := jobs[0].StartTime
	maxEnd := minStart

	for _, j := range jobs {
		if j.StartTime.Before(minStart) {
			minStart = j.StartTime
		}

		end := j.EndTime
		if end.IsZero() {
			end = time.Now()
		}
		if end.After(maxEnd) {
			maxEnd = end
		}
	}

	totalDuration := maxEnd.Sub(minStart)
	if totalDuration <= 0 {
		totalDuration = time.Second // prevent division by zero
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	idStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Width(20)

	colorCompleted := lipgloss.Color("42") // Green
	colorFailed := lipgloss.Color("196")   // Red
	colorRunning := lipgloss.Color("226")  // Yellow
	colorCanceled := lipgloss.Color("244") // Gray
	colorPending := lipgloss.Color("240")  // Dark Gray

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Execution Timeline (Total Window: %s)", totalDuration.Round(time.Second))))
	fmt.Fprintln(stdout, "")

	chartWidth := 50.0

	for _, j := range jobs {
		end := j.EndTime
		if end.IsZero() {
			end = time.Now()
		}

		startOffset := j.StartTime.Sub(minStart)
		jobDuration := end.Sub(j.StartTime)

		// Calculate proportional lengths
		offsetChars := int((float64(startOffset) / float64(totalDuration)) * chartWidth)
		barChars := int((float64(jobDuration) / float64(totalDuration)) * chartWidth)

		if barChars < 1 {
			barChars = 1 // Ensure at least 1 char width for very fast jobs
		}

		// Prevent overflow
		if offsetChars+barChars > int(chartWidth) {
			barChars = int(chartWidth) - offsetChars
		}

		// Choose color based on status
		var barColor lipgloss.Color
		switch strings.ToLower(j.Status) {
		case "completed", "success":
			barColor = colorCompleted
		case "failed", "error":
			barColor = colorFailed
		case "canceled", "cancelled":
			barColor = colorCanceled
		case "pending":
			barColor = colorPending
		default:
			barColor = colorRunning
		}

		barStyle := lipgloss.NewStyle().Foreground(barColor)

		// Build the visual bar
		pad := strings.Repeat(" ", offsetChars)
		bar := strings.Repeat("█", barChars)

		durStr := jobDuration.Round(time.Second).String()
		if durStr == "0s" {
			durStr = "<1s"
		}

		// Truncate ID if it's too long
		displayID := j.ID
		if len(displayID) > 19 {
			displayID = displayID[:16] + "..."
		}

		line := fmt.Sprintf("%-20s │%s%s %s (%s)",
			idStyle.Render(displayID),
			pad,
			barStyle.Render(bar),
			durStr,
			j.Status,
		)

		fmt.Fprintln(stdout, line)
	}

	fmt.Fprintln(stdout, "")
}
