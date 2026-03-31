package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

func summaryJobs(host string, format string) {
	urlStr := fmt.Sprintf("%s/jobs/summary", host)

	resp, err := http.Get(urlStr)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch summary: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var summary map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&summary); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Calculate total jobs for the title
	total := 0
	for _, count := range summary {
		total += count
	}

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(map[string]interface{}{
			"total":   total,
			"summary": summary,
		}); err != nil {
			fmt.Fprintf(stdout, "Failed to encode summary to JSON: %v\n", err)
			exitFunc(1)
		}
		return
	}

	// Styles
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Width(25).
		Foreground(lipgloss.Color("86"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Job Summary (%d total)", total)))
	fmt.Fprintln(stdout, "")

	if len(summary) == 0 {
		fmt.Fprintln(stdout, "No jobs found.")
		return
	}

	// Prepare status color map for a better UX
	colorMap := map[string]lipgloss.Style{
		"Completed":        lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true),  // Green
		"Failed":           lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true), // Red
		"Pending":          lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true), // Orange
		"Pending Approval": lipgloss.NewStyle().Foreground(lipgloss.Color("214")).Bold(true), // Orange
		"Spawning":         lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),  // Blue
		"Running":          lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),  // Blue
		"Active":           lipgloss.NewStyle().Foreground(lipgloss.Color("39")).Bold(true),  // Blue
		"Canceled":         lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true), // Gray
		"Skipped":          lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Bold(true), // Gray
		"Retrying":         lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true), // Yellow/Orange
	}

	// Print statuses ordered nicely (you can order or just print randomly)
	for status, count := range summary {
		lStyle := labelStyle
		if s, ok := colorMap[status]; ok {
			lStyle = s.Width(25)
		}
		fmt.Fprintf(stdout, "%s %s\n", lStyle.Render(status+":"), valueStyle.Render(fmt.Sprintf("%d", count)))
	}

	fmt.Fprintln(stdout, "")
}
