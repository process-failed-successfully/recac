package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

type FlakyJobStat struct {
	Summary      string  `json:"summary"`
	Occurrences  int     `json:"occurrences"`
	TotalRetries int     `json:"total_retries"`
	AvgRetries   float64 `json:"avg_retries"`
}

type FailedJobStat struct {
	Summary     string `json:"summary"`
	Occurrences int    `json:"occurrences"`
}

type ReliabilityStats struct {
	TotalJobs      int             `json:"total_jobs"`
	SuccessfulJobs int             `json:"successful_jobs"`
	FlakyJobs      int             `json:"flaky_jobs"`
	FailedJobs     int             `json:"failed_jobs"`
	SuccessRate    float64         `json:"success_rate"`
	FlakinessRate  float64         `json:"flakiness_rate"`
	FailureRate    float64         `json:"failure_rate"`
	TotalRetries   int             `json:"total_retries"`
	TopFlakyJobs   []FlakyJobStat  `json:"top_flaky_jobs"`
	TopFailingJobs []FailedJobStat `json:"top_failing_jobs"`
}

func analyzeReliability(host string, limit int, format string) {
	url := fmt.Sprintf("%s/jobs/analyze/reliability?limit=%d", host, limit)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch reliability stats: status %s, %s\n", resp.Status, strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var stats ReliabilityStats
	if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(stats); err != nil {
			fmt.Fprintf(stdout, "Failed to encode reliability stats to JSON: %v\n", err)
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
		Foreground(lipgloss.Color("86"))

	labelStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	successStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("46"))
	warnStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("214"))
	errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))

	fmt.Fprintln(stdout, titleStyle.Render("Pipeline Reliability Report"))
	fmt.Fprintln(stdout, "")

	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Total Evaluated Jobs:"), valueStyle.Render(fmt.Sprintf("%d", stats.TotalJobs)))
	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Successful Jobs:"), successStyle.Render(fmt.Sprintf("%d (%.2f%%)", stats.SuccessfulJobs, (float64(stats.SuccessfulJobs)/float64(stats.TotalJobs)*100))))
	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Flaky Jobs:"), warnStyle.Render(fmt.Sprintf("%d (%.2f%%)", stats.FlakyJobs, stats.FlakinessRate)))
	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Failed Jobs:"), errStyle.Render(fmt.Sprintf("%d (%.2f%%)", stats.FailedJobs, stats.FailureRate)))
	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Overall Success Rate (incl. Flaky):"), successStyle.Render(fmt.Sprintf("%.2f%%", stats.SuccessRate)))
	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Total Retries Performed:"), valueStyle.Render(fmt.Sprintf("%d", stats.TotalRetries)))

	fmt.Fprintln(stdout, "")

	if len(stats.TopFlakyJobs) > 0 {
		fmt.Fprintln(stdout, headerStyle.Render(fmt.Sprintf("Top %d Flaky Jobs (Succeeded eventually, but required retries)", limit)))
		fmt.Fprintf(stdout, "%-50s %-12s %-15s %-12s\n",
			labelStyle.Render("Summary"),
			labelStyle.Render("Occurrences"),
			labelStyle.Render("Total Retries"),
			labelStyle.Render("Avg Retries"),
		)
		for _, stat := range stats.TopFlakyJobs {
			fmt.Fprintf(stdout, "%-50s %-12d %-15d %-12.2f\n",
				limitString(stat.Summary, 48),
				stat.Occurrences,
				stat.TotalRetries,
				stat.AvgRetries,
			)
		}
		fmt.Fprintln(stdout, "")
	}

	if len(stats.TopFailingJobs) > 0 {
		fmt.Fprintln(stdout, headerStyle.Render(fmt.Sprintf("Top %d Failing Jobs (Failed completely)", limit)))
		fmt.Fprintf(stdout, "%-50s %-12s\n",
			labelStyle.Render("Summary"),
			labelStyle.Render("Occurrences"),
		)
		for _, stat := range stats.TopFailingJobs {
			fmt.Fprintf(stdout, "%-50s %-12d\n",
				limitString(stat.Summary, 48),
				stat.Occurrences,
			)
		}
		fmt.Fprintln(stdout, "")
	}

	if len(stats.TopFlakyJobs) == 0 && len(stats.TopFailingJobs) == 0 {
		fmt.Fprintln(stdout, successStyle.Render("Excellent! No flaky or failing jobs detected."))
	}
}
