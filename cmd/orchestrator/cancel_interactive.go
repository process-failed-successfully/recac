package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/lipgloss"
)

func cancelInteractive(host string) {
	fetchJobs := func(state string) ([]orchestrator.JobInfo, error) {
		u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
		if err != nil {
			return nil, fmt.Errorf("Failed to parse host URL: %w", err)
		}

		q := u.Query()
		q.Set("state", state)
		u.RawQuery = q.Encode()

		resp, err := http.Get(u.String())
		if err != nil {
			return nil, fmt.Errorf("Failed to connect to orchestrator at %s: %w", host, err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("Failed to fetch %s jobs: status %s", state, resp.Status)
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			return nil, fmt.Errorf("Failed to decode response: %w", err)
		}

		return jobs, nil
	}

	activeJobs, err := fetchJobs("active")
	if err != nil {
		fmt.Fprintln(stdout, err.Error())
		exitFunc(1)
		return
	}

	pendingJobs, err := fetchJobs("pending")
	if err != nil {
		fmt.Fprintln(stdout, err.Error())
		exitFunc(1)
		return
	}

	cancellableJobs := make([]orchestrator.JobInfo, 0, len(activeJobs)+len(pendingJobs))
	cancellableJobs = append(cancellableJobs, activeJobs...)
	cancellableJobs = append(cancellableJobs, pendingJobs...)

	if len(cancellableJobs) == 0 {
		fmt.Fprintln(stdout, "No active or pending jobs are currently cancellable.")
		return
	}

	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	labelStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(15)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Interactive Cancel (%d jobs)", len(cancellableJobs))))
	fmt.Fprintln(stdout, "")

	reader := bufio.NewReader(stdin)

	for _, job := range cancellableJobs {
		fmt.Fprintln(stdout, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render(strings.Repeat("-", 60)))

		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("ID:"), valueStyle.Render(job.ID))
		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Status:"), valueStyle.Render(job.Status))
		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Summary:"), valueStyle.Render(job.Summary))

		if job.WorkItem.Description != "" && job.WorkItem.Description != job.Summary {
			fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Description:"), valueStyle.Render(limitString(job.WorkItem.Description, 200)))
		}

		if len(job.WorkItem.Tags) > 0 {
			fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Tags:"), valueStyle.Render(strings.Join(job.WorkItem.Tags, ", ")))
		}

		if job.WorkItem.Priority != 0 {
			fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Priority:"), valueStyle.Render(fmt.Sprintf("%d", job.WorkItem.Priority)))
		}

		fmt.Fprintln(stdout, "")

		for {
			fmt.Fprintf(stdout, "Action for %s [c(ancel) / s(kip) / q(uit)] (default: s): ", job.ID)

			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintln(stdout, "\nError reading input. Exiting.")
				return
			}

			input = strings.TrimSpace(input)

			if input == "" || strings.EqualFold(input, "s") || strings.EqualFold(input, "skip") {
				fmt.Fprintf(stdout, "Skipping %s.\n", job.ID)
				break
			} else if strings.EqualFold(input, "c") || strings.EqualFold(input, "cancel") {
				cancelJob(host, job.ID, false)
				break
			} else if strings.EqualFold(input, "q") || strings.EqualFold(input, "quit") {
				fmt.Fprintln(stdout, "Exiting interactive cancel.")
				return
			} else {
				fmt.Fprintln(stdout, "Invalid input. Please enter 'c', 's', or 'q'.")
			}
		}
	}

	fmt.Fprintln(stdout, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render(strings.Repeat("-", 60)))
	fmt.Fprintln(stdout, "All cancellable jobs processed.")
}
