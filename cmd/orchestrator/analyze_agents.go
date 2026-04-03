package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/lipgloss"
)

func analyzeAgents(host string, limit int, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/agents", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("limit", fmt.Sprintf("%d", limit))
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
		fmt.Fprintf(stdout, "Failed to fetch agents analysis: status %s\n%s\n", resp.Status, body)
		exitFunc(1)
		return
	}

	var stats orchestrator.AgentStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(stats); err != nil {
			fmt.Fprintf(stdout, "Failed to encode agents stats to JSON: %v\n", err)
			exitFunc(1)
		}
		return
	}

	if len(stats.Agents) == 0 {
		fmt.Fprintln(stdout, "No valid completed jobs with agent data found.")
		return
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#10B981")). // Emerald green
		Padding(0, 1).
		MarginBottom(1)

	sectionTitleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("212")).
		MarginTop(1).
		MarginBottom(1)

	tableHeaderStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("252")).
		PaddingRight(2)

	rowStyle := lipgloss.NewStyle().PaddingRight(2)

	fmt.Fprintln(stdout, titleStyle.Render("AI Agent Performance Analysis"))
	fmt.Fprintln(stdout, sectionTitleStyle.Render("Agent Model Metrics"))

	fmt.Fprintf(stdout, "%-15s %-20s %-10s %-12s %-15s %-15s %-12s\n",
		tableHeaderStyle.Render("Provider"),
		tableHeaderStyle.Render("Model"),
		tableHeaderStyle.Render("Jobs"),
		tableHeaderStyle.Render("Success Rate"),
		tableHeaderStyle.Render("Avg Duration"),
		tableHeaderStyle.Render("Avg Cost/Job"),
		tableHeaderStyle.Render("Total Cost"),
	)

	for _, stat := range stats.Agents {
		durationStr := "N/A"
		if stat.AverageDuration > 0 {
			durationStr = stat.AverageDuration.Round(time.Second).String()
		}

		successRateStr := fmt.Sprintf("%.1f%%", stat.SuccessRate*100)

		fmt.Fprintf(stdout, "%-15s %-20s %-10s %-12s %-15s %-15s %-12s\n",
			rowStyle.Render(stat.AgentProvider),
			rowStyle.Render(stat.AgentModel),
			rowStyle.Render(fmt.Sprintf("%d", stat.TotalJobs)),
			rowStyle.Render(successRateStr),
			rowStyle.Render(durationStr),
			rowStyle.Render(fmt.Sprintf("$%.4f", stat.AverageCost)),
			rowStyle.Render(fmt.Sprintf("$%.4f", stat.TotalCost)),
		)
	}
	fmt.Fprintln(stdout, "")
}
