package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

func listGroups(host string, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/groups", host))
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
		fmt.Fprintf(stdout, "Failed to fetch groups: status %s\n", resp.Status)
		exitFunc(1)
		return
	}

	var groups []orchestrator.GroupInfo
	if err := json.NewDecoder(resp.Body).Decode(&groups); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(groups); err != nil {
			fmt.Fprintf(stdout, "Failed to encode groups to JSON: %v\n", err)
			exitFunc(1)
		}
		return
	}

	if len(groups) == 0 {
		fmt.Fprintln(stdout, "No concurrency groups found.")
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

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Concurrency Groups (%d)", len(groups))))
	fmt.Fprintln(stdout, "")

	// Table Header
	fmt.Fprintf(stdout, "%-30s %-15s %-15s %-15s\n",
		headerStyle.Render("Name"),
		headerStyle.Render("Active Jobs"),
		headerStyle.Render("Pending Jobs"),
		headerStyle.Render("Paused"),
	)

	for _, group := range groups {
		fmt.Fprintf(stdout, "%-30s %-15s %-15s %-15s\n",
			rowStyle.Render(limitString(group.Name, 28)),
			rowStyle.Render(fmt.Sprintf("%d", group.ActiveJobs)),
			rowStyle.Render(fmt.Sprintf("%d", group.PendingJobs)),
			rowStyle.Render(fmt.Sprintf("%t", group.Paused)),
		)
	}
}
