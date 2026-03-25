package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

func analyzeFailures(host string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("state", "all")
	q.Set("status", "Failed")
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
		fmt.Fprintf(stdout, "Failed to fetch failed jobs: %s\n", strings.TrimSpace(string(body)))
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
		fmt.Fprintln(stdout, "No failed jobs found.")
		return
	}

	// Group jobs by summary
	summaryMap := make(map[string][]string) // Summary -> []JobIDs
	for _, job := range jobs {
		summary := strings.TrimSpace(job.Summary)
		if summary == "" {
			summary = "<empty summary>"
		}
		summaryMap[summary] = append(summaryMap[summary], job.ID)
	}

	// Prepare to display sorted by count (descending), then alphabetically by summary
	type summaryGroup struct {
		summary string
		jobIDs  []string
		count   int
	}

	var groups []summaryGroup
	for summary, ids := range summaryMap {
		groups = append(groups, summaryGroup{
			summary: summary,
			jobIDs:  ids,
			count:   len(ids),
		})
	}

	sort.Slice(groups, func(i, j int) bool {
		if groups[i].count != groups[j].count {
			return groups[i].count > groups[j].count
		}
		return groups[i].summary < groups[j].summary
	})

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

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Failed Jobs Analysis (%d total)", len(jobs))))
	fmt.Fprintln(stdout, "")

	// Table Header
	fmt.Fprintf(stdout, "%-10s %-50s %-40s\n",
		headerStyle.Render("Count"),
		headerStyle.Render("Error Signature (Summary)"),
		headerStyle.Render("Job IDs"),
	)

	for _, g := range groups {
		countStr := fmt.Sprintf("%d", g.count)

		// Join job IDs, truncate if too long
		jobIDsStr := strings.Join(g.jobIDs, ", ")
		if len(jobIDsStr) > 38 {
			jobIDsStr = jobIDsStr[:35] + "..."
		}

		fmt.Fprintf(stdout, "%-10s %-50s %-40s\n",
			rowStyle.Render(countStr),
			rowStyle.Render(limitString(g.summary, 48)),
			rowStyle.Render(jobIDsStr),
		)
	}
}
