package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

type TagCostStat struct {
	Tag   string  `json:"tag"`
	Cost  float64 `json:"cost"`
	Count int     `json:"count"`
}

type ModelCostStat struct {
	Model string  `json:"model"`
	Cost  float64 `json:"cost"`
	Count int     `json:"count"`
}

type CostStats struct {
	TotalJobs        int                  `json:"total_jobs"`
	TotalCost        float64              `json:"total_cost"`
	TotalTokens      float64              `json:"total_tokens"`
	CostByTag        []TagCostStat        `json:"cost_by_tag"`
	CostByModel      []ModelCostStat      `json:"cost_by_model"`
	TopExpensiveJobs []orchestrator.JobInfo `json:"top_expensive_jobs"`
}

func analyzeCosts(host string, limit int, format string) {
	url := fmt.Sprintf("%s/jobs/analyze/costs?limit=%d", host, limit)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch cost stats: status %s, %s\n", resp.Status, strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var stats CostStats
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

	// Text UI Formatting
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

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Pipeline Cost Analysis (%d evaluated jobs)", stats.TotalJobs)))
	fmt.Fprintln(stdout, "")

	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Total Cost:"), valueStyle.Render(fmt.Sprintf("$%.4f", stats.TotalCost)))
	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Total Tokens:"), valueStyle.Render(fmt.Sprintf("%.0f", stats.TotalTokens)))
	if stats.TotalJobs > 0 {
		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Avg Cost / Job:"), valueStyle.Render(fmt.Sprintf("$%.4f", stats.TotalCost/float64(stats.TotalJobs))))
	}

	fmt.Fprintln(stdout, "")

	if len(stats.CostByModel) > 0 {
		fmt.Fprintln(stdout, titleStyle.Render("Cost by Model"))
		fmt.Fprintln(stdout, "")
		fmt.Fprintf(stdout, "%-30s %-10s %-15s %-15s\n",
			headerStyle.Render("Model"),
			headerStyle.Render("Count"),
			headerStyle.Render("Total Cost"),
			headerStyle.Render("Avg Cost/Job"),
		)
		for _, stat := range stats.CostByModel {
			fmt.Fprintf(stdout, "%-30s %-10d %-15s %-15s\n",
				rowStyle.Render(limitString(stat.Model, 28)),
				stat.Count,
				rowStyle.Render(fmt.Sprintf("$%.4f", stat.Cost)),
				rowStyle.Render(fmt.Sprintf("$%.4f", stat.Cost/float64(stat.Count))),
			)
		}
		fmt.Fprintln(stdout, "")
	}

	if len(stats.CostByTag) > 0 {
		fmt.Fprintln(stdout, titleStyle.Render("Cost by Tag"))
		fmt.Fprintln(stdout, "")
		fmt.Fprintf(stdout, "%-30s %-10s %-15s %-15s\n",
			headerStyle.Render("Tag"),
			headerStyle.Render("Count"),
			headerStyle.Render("Total Cost"),
			headerStyle.Render("Avg Cost/Job"),
		)
		for _, stat := range stats.CostByTag {
			fmt.Fprintf(stdout, "%-30s %-10d %-15s %-15s\n",
				rowStyle.Render(limitString(stat.Tag, 28)),
				stat.Count,
				rowStyle.Render(fmt.Sprintf("$%.4f", stat.Cost)),
				rowStyle.Render(fmt.Sprintf("$%.4f", stat.Cost/float64(stat.Count))),
			)
		}
		fmt.Fprintln(stdout, "")
	}

	if len(stats.TopExpensiveJobs) > 0 {
		fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Top %d Expensive Jobs", limit)))
		fmt.Fprintln(stdout, "")

		fmt.Fprintf(stdout, "%-15s %-40s %-15s %-10s %-15s\n",
			headerStyle.Render("ID"),
			headerStyle.Render("Summary"),
			headerStyle.Render("Status"),
			headerStyle.Render("Cost"),
			headerStyle.Render("Tokens"),
		)

		for _, job := range stats.TopExpensiveJobs {
			cost := 0.0
			if c, ok := job.Metrics["cost"]; ok {
				cost = c
			}
			tokens := 0.0
			if t, ok := job.Metrics["tokens"]; ok {
				tokens = t
			}

			fmt.Fprintf(stdout, "%-15s %-40s %-15s %-10s %-15s\n",
				rowStyle.Render(limitString(job.ID, 13)),
				rowStyle.Render(limitString(job.Summary, 38)),
				rowStyle.Render(limitString(job.Status, 13)),
				rowStyle.Render(fmt.Sprintf("$%.4f", cost)),
				rowStyle.Render(fmt.Sprintf("%.0f", tokens)),
			)
		}
		fmt.Fprintln(stdout, "")
	}

	if len(stats.TopExpensiveJobs) == 0 {
		fmt.Fprintln(stdout, successStyle.Render("No jobs with associated costs evaluated."))
	}
}
