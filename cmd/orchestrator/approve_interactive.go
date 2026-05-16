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

func approveInteractive(host string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse host URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	q.Set("state", "pending")
	u.RawQuery = q.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(stdout, "Failed to fetch pending jobs: status %s\n", resp.Status)
		exitFunc(1)
		return
	}

	var allJobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&allJobs); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	var pendingApproval []orchestrator.JobInfo
	for _, job := range allJobs {
		if job.Status == "Pending Approval" {
			pendingApproval = append(pendingApproval, job)
		}
	}

	if len(pendingApproval) == 0 {
		fmt.Fprintln(stdout, "No jobs are currently pending approval.")
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

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Interactive Approval (%d jobs)", len(pendingApproval))))
	fmt.Fprintln(stdout, "")

	reader := bufio.NewReader(stdin)

	for _, job := range pendingApproval {
		fmt.Fprintln(stdout, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render(strings.Repeat("-", 60)))

		fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("ID:"), valueStyle.Render(job.ID))
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

		if len(job.WorkItem.DependsOn) > 0 {
			fmt.Fprintf(stdout, "%s %s\n", labelStyle.Render("Depends On:"), valueStyle.Render(strings.Join(job.WorkItem.DependsOn, ", ")))
		}

		fmt.Fprintln(stdout, "")

		for {
			fmt.Fprintf(stdout, "Action for %s [a(pprove) / s(kip) / c(ancel) / q(uit)] (default: s): ", job.ID)

			input, err := reader.ReadString('\n')
			if err != nil {
				fmt.Fprintln(stdout, "\nError reading input. Exiting.")
				return
			}

			// ⚡ Bolt: Use strings.EqualFold to avoid allocating a new string with strings.ToLower
			input = strings.TrimSpace(input)

			if input == "" || strings.EqualFold(input, "s") || strings.EqualFold(input, "skip") {
				skipJob(host, job.ID)
				break
			} else if strings.EqualFold(input, "a") || strings.EqualFold(input, "approve") || strings.EqualFold(input, "y") || strings.EqualFold(input, "yes") {
				approveJob(host, job.ID)
				break
			} else if strings.EqualFold(input, "c") || strings.EqualFold(input, "cancel") {
				cancelJob(host, job.ID, false)
				break
			} else if strings.EqualFold(input, "q") || strings.EqualFold(input, "quit") {
				fmt.Fprintln(stdout, "Exiting interactive approval.")
				return
			} else {
				fmt.Fprintln(stdout, "Invalid input. Please enter 'a', 's', 'c', or 'q'.")
			}
		}
	}

	fmt.Fprintln(stdout, lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("214")).Render(strings.Repeat("-", 60)))
	fmt.Fprintln(stdout, "All pending approval jobs processed.")
}
