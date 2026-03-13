package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type LogMatch struct {
	LineNumber int    `json:"line_number"`
	Text       string `json:"text"`
}

type JobLogResult struct {
	JobID   string     `json:"job_id"`
	Summary string     `json:"summary"`
	Status  string     `json:"status"`
	Matches []LogMatch `json:"matches"`
}

func searchLogs(host, query, tag, status string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/search/logs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("q", query)
	if tag != "" {
		q.Set("tag", tag)
	}
	if status != "" {
		q.Set("status", status)
	}
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
		fmt.Fprintf(stdout, "Failed to search logs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var results []JobLogResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		fmt.Fprintf(stdout, "Failed to decode search results: %v\n", err)
		exitFunc(1)
		return
	}

	if len(results) == 0 {
		fmt.Fprintln(stdout, "No matching logs found.")
		return
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	jobStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	statusStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	lineNumStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	textStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("250"))

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Log Search Results (query: %q)", query)))
	fmt.Fprintln(stdout, "")

	for _, job := range results {
		fmt.Fprintf(stdout, "Job: %s (%s)\n", jobStyle.Render(job.JobID), statusStyle.Render(job.Status))
		fmt.Fprintf(stdout, "Summary: %s\n", job.Summary)

		for _, match := range job.Matches {
			fmt.Fprintf(stdout, "  %s %s\n",
				lineNumStyle.Render(fmt.Sprintf("Line %d:", match.LineNumber)),
				textStyle.Render(strings.TrimSpace(match.Text)),
			)
		}
		fmt.Fprintln(stdout, "")
	}
}
