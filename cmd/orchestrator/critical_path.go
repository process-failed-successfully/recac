package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

func printCriticalPath(host string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("state", "all") // Get all jobs (active + history)
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch jobs: status %s\n%s\n", resp.Status, strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var allJobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&allJobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if len(allJobs) == 0 {
		fmt.Fprintln(stdout, "No jobs available for critical path analysis.")
		return
	}

	path, totalDur := orchestrator.CalculateCriticalPath(allJobs)

	if len(path) == 0 {
		fmt.Fprintln(stdout, "No started jobs found to analyze.")
		return
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		MarginBottom(1)

	jobStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	durStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	arrowStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250")).
		Bold(true)

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Critical Path Analysis (Total Critical Duration: %s)", totalDur.Round(time.Second))))

	for i, j := range path {
		end := j.EndTime
		if end.IsZero() {
			end = time.Now()
		}
		dur := end.Sub(j.StartTime).Round(time.Second)

		fmt.Fprintf(stdout, "%s %s %s\n",
			jobStyle.Render(j.ID),
			durStyle.Render(fmt.Sprintf("[%s]", dur.String())),
			statusStyle.Render(fmt.Sprintf("(%s)", j.Status)),
		)

		if i < len(path)-1 {
			fmt.Fprintln(stdout, arrowStyle.Render("   ↓"))
		}
	}
	fmt.Fprintln(stdout, "")
}
