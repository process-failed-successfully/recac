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

func runDiagnose(host string) {
	url := fmt.Sprintf("%s/diagnose", host)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch diagnostics: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var report orchestrator.DiagnosticReport
	if err := json.NewDecoder(resp.Body).Decode(&report); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86"))

	errorStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("196"))

	jobStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	fmt.Fprintln(stdout, titleStyle.Render("Diagnostic Report"))
	fmt.Fprintln(stdout, "")

	if len(report.UnresolvableJobs) == 0 && len(report.DeadlockedJobs) == 0 {
		fmt.Fprintln(stdout, lipgloss.NewStyle().Foreground(lipgloss.Color("46")).Render("No issues found! The system is healthy."))
		return
	}

	if len(report.UnresolvableJobs) > 0 {
		fmt.Fprintf(stdout, "%s\n", headerStyle.Render(fmt.Sprintf("Unresolvable Jobs (%d)", len(report.UnresolvableJobs))))
		for _, uj := range report.UnresolvableJobs {
			fmt.Fprintf(stdout, "  %s\n", jobStyle.Render(uj.JobID))
			if len(uj.MissingDeps) > 0 {
				fmt.Fprintf(stdout, "    %s %s\n", errorStyle.Render("Missing:"), strings.Join(uj.MissingDeps, ", "))
			}
			if len(uj.FailedDeps) > 0 {
				fmt.Fprintf(stdout, "    %s  %s\n", errorStyle.Render("Failed:"), strings.Join(uj.FailedDeps, ", "))
			}
			if len(uj.CanceledDeps) > 0 {
				fmt.Fprintf(stdout, "    %s %s\n", errorStyle.Render("Canceled:"), strings.Join(uj.CanceledDeps, ", "))
			}
		}
		fmt.Fprintln(stdout, "")
	}

	if len(report.DeadlockedJobs) > 0 {
		fmt.Fprintf(stdout, "%s\n", headerStyle.Render(fmt.Sprintf("Deadlocks / Cyclic Dependencies (%d)", len(report.DeadlockedJobs))))
		for _, dj := range report.DeadlockedJobs {
			fmt.Fprintf(stdout, "  %s\n", jobStyle.Render(dj.JobID))

			// Format cycle nicely A -> B -> C -> A
			cycleStr := strings.Join(dj.Cycle, " -> ")
			fmt.Fprintf(stdout, "    %s %s\n", errorStyle.Render("Cycle:"), cycleStr)
		}
		fmt.Fprintln(stdout, "")
	}
}
