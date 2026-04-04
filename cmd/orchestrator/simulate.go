package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
	"recac/internal/orchestrator"
)

func simulatePipelineFileCmd(host string, pipelineFile string, targetJob string) {
	content, err := os.ReadFile(pipelineFile)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read pipeline file: %v\n", err)
		exitFunc(1)
		return
	}

	url := fmt.Sprintf("%s/simulate/pipeline", host)
	if targetJob != "" {
		url = fmt.Sprintf("%s?target=%s", url, targetJob)
	}

	req, err := http.NewRequest(http.MethodPost, url, bytes.NewBuffer(content))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/yaml")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch pipeline simulation report: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var report orchestrator.SimulationReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	printSimulationReport(report)
}

func printSimulationReport(report orchestrator.SimulationReport) {
	if report.TotalJobs == 0 {
		fmt.Fprintln(stdout, lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("No jobs to simulate. Simulation complete: 0s."))
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
		Foreground(lipgloss.Color("86")).
		Width(25)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	warnStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("196"))

	fmt.Fprintln(stdout, titleStyle.Render("Simulation Report"))
	fmt.Fprintln(stdout, "")

	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Estimated Total Time:"), valueStyle.Render(time.Duration(report.EstimatedTotalTimeMs*1e6).Round(time.Second).String()))
	fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Jobs Processed:"), valueStyle.Render(fmt.Sprintf("%d / %d", report.JobsProcessed, report.TotalJobs)))

	if report.FinalBottleneckJob != "" {
		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Final Bottleneck Job:"), valueStyle.Render(report.FinalBottleneckJob))
	}

	if report.Deadlocks > 0 {
		fmt.Fprintln(stdout, "")
		fmt.Fprintf(stdout, "%s\n", warnStyle.Render(fmt.Sprintf("WARNING: %d jobs could not be processed due to unresolved/circular dependencies!", report.Deadlocks)))
	}
	fmt.Fprintln(stdout, "")
}

func simulateExecution(host string) {
	url := fmt.Sprintf("%s/simulate", host)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch simulation report: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var report orchestrator.SimulationReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	printSimulationReport(report)
}
