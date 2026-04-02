package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/lipgloss"
)

func analyzeCosts(host string, limit int, format string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/analyze/costs", host))
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
		fmt.Fprintf(stdout, "Failed to fetch cost analysis: status %s\n%s\n", resp.Status, body)
		exitFunc(1)
		return
	}

	var stats orchestrator.CostStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(stats); err != nil {
			fmt.Fprintf(stdout, "Failed to encode cost stats to JSON: %v\n", err)
			exitFunc(1)
		}
		return
	}

	if stats.TotalStats.TotalJobs == 0 {
		fmt.Fprintln(stdout, "No valid completed jobs with cost data found.")
		return
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1).
		MarginBottom(1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(25)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

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

	fmt.Fprintln(stdout, titleStyle.Render("AI Cost Analysis"))

	// Print Total Stats
	printField := func(label, value string) {
		fmt.Fprintf(stdout, "%s %s\n", headerStyle.Render(label+":"), valueStyle.Render(value))
	}

	printField("Total Evaluated Jobs", fmt.Sprintf("%d", stats.TotalStats.TotalJobs))
	printField("Total Cost", fmt.Sprintf("$%.4f", stats.TotalStats.TotalCost))
	printField("Total Prompt Tokens", fmt.Sprintf("%.0f", stats.TotalStats.TotalTokensPrompt))
	printField("Total Completion Tokens", fmt.Sprintf("%.0f", stats.TotalStats.TotalTokensCompletion))

	// Print Tag Stats
	if len(stats.TagStats) > 0 {
		fmt.Fprintln(stdout, sectionTitleStyle.Render("Cost by Tag"))
		fmt.Fprintf(stdout, "%-30s %-15s %-15s\n",
			tableHeaderStyle.Render("Tag"),
			tableHeaderStyle.Render("Jobs"),
			tableHeaderStyle.Render("Total Cost"),
		)
		for _, stat := range stats.TagStats {
			fmt.Fprintf(stdout, "%-30s %-15s %-15s\n",
				rowStyle.Render(stat.Tag),
				rowStyle.Render(fmt.Sprintf("%d", stat.JobsCount)),
				rowStyle.Render(fmt.Sprintf("$%.4f", stat.Cost)),
			)
		}
	}

	// Print Model Stats
	if len(stats.ModelStats) > 0 {
		fmt.Fprintln(stdout, sectionTitleStyle.Render("Cost by Model"))
		fmt.Fprintf(stdout, "%-30s %-15s %-15s\n",
			tableHeaderStyle.Render("Model"),
			tableHeaderStyle.Render("Jobs"),
			tableHeaderStyle.Render("Total Cost"),
		)
		for _, stat := range stats.ModelStats {
			fmt.Fprintf(stdout, "%-30s %-15s %-15s\n",
				rowStyle.Render(stat.Model),
				rowStyle.Render(fmt.Sprintf("%d", stat.JobsCount)),
				rowStyle.Render(fmt.Sprintf("$%.4f", stat.Cost)),
			)
		}
	}

	// Print Top Expensive Jobs
	if len(stats.TopExpensiveJobs) > 0 {
		fmt.Fprintln(stdout, sectionTitleStyle.Render(fmt.Sprintf("Top %d Most Expensive Jobs", len(stats.TopExpensiveJobs))))
		fmt.Fprintf(stdout, "%-25s %-40s %-15s\n",
			tableHeaderStyle.Render("ID"),
			tableHeaderStyle.Render("Summary"),
			tableHeaderStyle.Render("Cost"),
		)
		for _, job := range stats.TopExpensiveJobs {
			summary := job.Summary
			if len(summary) > 38 {
				summary = summary[:35] + "..."
			}

			cost := 0.0
			if c, ok := job.Metrics["cost_usd"]; ok {
				cost = c
			}

			fmt.Fprintf(stdout, "%-25s %-40s %-15s\n",
				rowStyle.Render(job.ID),
				rowStyle.Render(summary),
				rowStyle.Render(fmt.Sprintf("$%.4f", cost)),
			)
		}
	}
	fmt.Fprintln(stdout, "")
}
