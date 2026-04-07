package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

func analyzeAnomalies(host string, limit int, format string) {
	url := fmt.Sprintf("%s/jobs/analyze/anomalies", host)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch anomalies analysis: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var anomalies []orchestrator.AnomalyReport
	if err := json.NewDecoder(resp.Body).Decode(&anomalies); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Sort anomalies by most deviant duration first
	sort.Slice(anomalies, func(i, j int) bool {
		return anomalies[i].DurationDev > anomalies[j].DurationDev
	})

	if limit > 0 && len(anomalies) > limit {
		anomalies = anomalies[:limit]
	}

	if format == "json" {
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(anomalies); err != nil {
			fmt.Fprintf(stdout, "Failed to encode anomalies to JSON: %v\n", err)
			exitFunc(1)
		}
		return
	}

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

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Anomaly Analysis (%d top anomalies)", len(anomalies))))
	fmt.Fprintln(stdout, "")

	if len(anomalies) == 0 {
		fmt.Fprintln(stdout, "No anomalies found.")
		return
	}

	// Table Header
	fmt.Fprintf(stdout, "%-15s %-20s %-12s %-15s %-15s %-10s %-10s\n",
		headerStyle.Render("Job ID"),
		headerStyle.Render("Model"),
		headerStyle.Render("Status"),
		headerStyle.Render("Duration"),
		headerStyle.Render("Cost"),
		headerStyle.Render("Dur Dev"),
		headerStyle.Render("Cost Dev"),
	)

	for _, a := range anomalies {
		durDevStr := fmt.Sprintf("%.2fσ", a.DurationDev)
		if a.DurationDev == 0 {
			durDevStr = "-"
		}

		costDevStr := fmt.Sprintf("%.2fσ", a.CostDev)
		if a.CostDev == 0 {
			costDevStr = "-"
		}

		fmt.Fprintf(stdout, "%-15s %-20s %-12s %-15s %-15s %-10s %-10s\n",
			rowStyle.Render(limitString(a.JobID, 13)),
			rowStyle.Render(limitString(a.Model, 18)),
			rowStyle.Render(limitString(a.Status, 10)),
			rowStyle.Render(a.Duration.String()),
			rowStyle.Render(fmt.Sprintf("$%.4f", a.Cost)),
			rowStyle.Render(durDevStr),
			rowStyle.Render(costDevStr),
		)
	}
	fmt.Fprintln(stdout, "")
}
