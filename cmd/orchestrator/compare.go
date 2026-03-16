package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

// compareJobs fetches two jobs from the orchestrator and renders a side-by-side comparison table.
func compareJobs(host, jobsStr string) {
	// Parse jobsStr
	parts := strings.Split(jobsStr, ",")
	if len(parts) != 2 {
		fmt.Fprintf(stdout, "Error: --compare-jobs expects exactly two job IDs separated by a comma (e.g., job1,job2)\n")
		exitFunc(1)
		return
	}

	id1 := strings.TrimSpace(parts[0])
	id2 := strings.TrimSpace(parts[1])

	if id1 == "" || id2 == "" {
		fmt.Fprintf(stdout, "Error: Job IDs cannot be empty\n")
		exitFunc(1)
		return
	}

	// Fetch job 1
	job1, err := fetchJob(host, id1)
	if err != nil {
		fmt.Fprintf(stdout, "Error fetching job %s: %v\n", id1, err)
		exitFunc(1)
		return
	}

	// Fetch job 2
	job2, err := fetchJob(host, id2)
	if err != nil {
		fmt.Fprintf(stdout, "Error fetching job %s: %v\n", id2, err)
		exitFunc(1)
		return
	}

	renderComparison(job1, job2)
}

func fetchJob(host, jobID string) (*orchestrator.JobInfo, error) {
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
	if err != nil {
		return nil, fmt.Errorf("failed to connect to orchestrator: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %s: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &job, nil
}

func renderComparison(job1, job2 *orchestrator.JobInfo) {
	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		MarginBottom(1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Width(30).
		PaddingRight(2)

	diffStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Width(30).
		PaddingRight(2)

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Comparing: %s vs %s", job1.ID, job2.ID)))

	printRow := func(label, v1, v2 string) {
		s1 := valueStyle
		s2 := valueStyle
		if v1 != v2 {
			s1 = diffStyle
			s2 = diffStyle
		}
		fmt.Fprintf(stdout, "%s %s | %s\n", headerStyle.Render(label+":"), s1.Render(limitString(v1, 28)), s2.Render(limitString(v2, 28)))
	}

	// Get durations
	dur1 := "N/A"
	if !job1.StartTime.IsZero() {
		if !job1.EndTime.IsZero() {
			dur1 = job1.EndTime.Sub(job1.StartTime).Round(time.Second).String()
		} else {
			dur1 = time.Since(job1.StartTime).Round(time.Second).String() + " (running)"
		}
	}

	dur2 := "N/A"
	if !job2.StartTime.IsZero() {
		if !job2.EndTime.IsZero() {
			dur2 = job2.EndTime.Sub(job2.StartTime).Round(time.Second).String()
		} else {
			dur2 = time.Since(job2.StartTime).Round(time.Second).String() + " (running)"
		}
	}

	printRow("ID", job1.ID, job2.ID)
	printRow("Summary", job1.Summary, job2.Summary)
	printRow("Status", job1.Status, job2.Status)
	printRow("Agent Provider", job1.WorkItem.AgentProvider, job2.WorkItem.AgentProvider)
	printRow("Agent Model", job1.WorkItem.AgentModel, job2.WorkItem.AgentModel)
	printRow("Duration", dur1, dur2)

	// Outputs
	fmt.Fprintln(stdout, "\n"+lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("--- Outputs ---"))
	allOutputKeys := make(map[string]bool)
	for k := range job1.Outputs {
		allOutputKeys[k] = true
	}
	for k := range job2.Outputs {
		allOutputKeys[k] = true
	}
	if len(allOutputKeys) == 0 {
		fmt.Fprintln(stdout, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No outputs for either job."))
	} else {
		var keys []string
		for k := range allOutputKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v1, ok1 := job1.Outputs[k]
			if !ok1 {
				v1 = "<missing>"
			}
			v2, ok2 := job2.Outputs[k]
			if !ok2 {
				v2 = "<missing>"
			}
			printRow(k, v1, v2)
		}
	}

	// Metrics
	fmt.Fprintln(stdout, "\n"+lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("--- Metrics ---"))
	allMetricKeys := make(map[string]bool)
	for k := range job1.Metrics {
		allMetricKeys[k] = true
	}
	for k := range job2.Metrics {
		allMetricKeys[k] = true
	}
	if len(allMetricKeys) == 0 {
		fmt.Fprintln(stdout, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("No metrics for either job."))
	} else {
		var keys []string
		for k := range allMetricKeys {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			v1 := "<missing>"
			if val, ok := job1.Metrics[k]; ok {
				v1 = fmt.Sprintf("%.2f", val)
			}
			v2 := "<missing>"
			if val, ok := job2.Metrics[k]; ok {
				v2 = fmt.Sprintf("%.2f", val)
			}
			printRow(k, v1, v2)
		}
	}
}
