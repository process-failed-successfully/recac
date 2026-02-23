package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"recac/internal/orchestrator"
	"recac/internal/tui"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage orchestrator jobs",
	Long:  `Manage and monitor jobs running on the RECAC Orchestrator.`,
}

var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active and completed jobs",
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("job.host")
		history := viper.GetBool("job.history")
		listJobs(host, history, cmd.OutOrStdout())
	},
}

var jobMonitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Launch the TUI dashboard to monitor jobs",
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("job.host")
		if err := tui.StartDashboard(host); err != nil {
			fmt.Fprintf(cmd.ErrOrStderr(), "Dashboard failed: %v\n", err)
			os.Exit(1)
		}
	},
}

var jobInfoCmd = &cobra.Command{
	Use:   "info [id]",
	Short: "Inspect a specific job by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("job.host")
		inspectJob(host, args[0], cmd.OutOrStdout())
	},
}

var jobLogsCmd = &cobra.Command{
	Use:   "logs [id]",
	Short: "Get logs for a specific job ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("job.host")
		getLogs(host, args[0], cmd.OutOrStdout())
	},
}

var jobSubmitCmd = &cobra.Command{
	Use:   "submit [file]",
	Short: "Submit a job from a JSON file",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("job.host")
		wait := viper.GetBool("job.wait")
		submitJob(host, args[0], wait, cmd.OutOrStdout())
	},
}

var jobCancelCmd = &cobra.Command{
	Use:   "cancel [id]",
	Short: "Cancel a running job by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("job.host")
		cancelJob(host, args[0], cmd.OutOrStdout())
	},
}

var jobRetryCmd = &cobra.Command{
	Use:   "retry [id]",
	Short: "Retry a completed job by ID",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("job.host")
		retryJob(host, args[0], cmd.OutOrStdout())
	},
}

var jobRetryFailedCmd = &cobra.Command{
	Use:   "retry-failed",
	Short: "Retry all failed jobs from history",
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("job.host")
		retryFailedJobs(host, cmd.OutOrStdout())
	},
}

func init() {
	jobCmd.PersistentFlags().String("host", "http://localhost:2112", "Orchestrator host URL")
	viper.BindPFlag("job.host", jobCmd.PersistentFlags().Lookup("host"))
	viper.BindEnv("job.host", "RECAC_ORCHESTRATOR_HOST")

	jobListCmd.Flags().Bool("history", false, "Include completed jobs")
	viper.BindPFlag("job.history", jobListCmd.Flags().Lookup("history"))

	jobSubmitCmd.Flags().Bool("wait", false, "Wait for job completion and stream logs")
	viper.BindPFlag("job.wait", jobSubmitCmd.Flags().Lookup("wait"))

	jobCmd.AddCommand(jobListCmd)
	jobCmd.AddCommand(jobMonitorCmd)
	jobCmd.AddCommand(jobInfoCmd)
	jobCmd.AddCommand(jobLogsCmd)
	jobCmd.AddCommand(jobSubmitCmd)
	jobCmd.AddCommand(jobCancelCmd)
	jobCmd.AddCommand(jobRetryCmd)
	jobCmd.AddCommand(jobRetryFailedCmd)

	rootCmd.AddCommand(jobCmd)
}

func listJobs(host string, history bool, out io.Writer) {
	url := fmt.Sprintf("%s/jobs", host)
	if history {
		url += "?state=all"
	}

	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(out, "Failed to connect to orchestrator at %s: %v\n", host, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(out, "Failed to fetch jobs: status %s\n", resp.Status)
		return
	}

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
		fmt.Fprintf(out, "Failed to decode response: %v\n", err)
		return
	}

	if len(jobs) == 0 {
		// Use dashboard style centered message or just text
		fmt.Fprintln(out, "No active jobs.")
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

	title := "Active Jobs"
	if history {
		title = "All Jobs (Active & History)"
	}
	fmt.Fprintln(out, titleStyle.Render(fmt.Sprintf("%s (%d)", title, len(jobs))))
	fmt.Fprintln(out, "")

	// Table Header
	fmt.Fprintf(out, "%-15s %-40s %-15s %-20s\n",
		headerStyle.Render("ID"),
		headerStyle.Render("Summary"),
		headerStyle.Render("Status"),
		headerStyle.Render("Duration"),
	)

	for _, job := range jobs {
		duration := time.Since(job.StartTime).Round(time.Second).String()
		if !job.EndTime.IsZero() {
			duration = job.EndTime.Sub(job.StartTime).Round(time.Second).String()
		}

		statusColor := "252" // Default grey
		switch job.Status {
		case "Running", "Spawning":
			statusColor = "39" // Blue
		case "Completed":
			statusColor = "42" // Green
		case "Failed":
			statusColor = "196" // Red
		}
		statusStyle := rowStyle.Copy().Foreground(lipgloss.Color(statusColor))

		fmt.Fprintf(out, "%-15s %-40s %-15s %-20s\n",
			rowStyle.Render(job.ID),
			rowStyle.Render(limitString(job.Summary, 38)),
			statusStyle.Render(job.Status),
			rowStyle.Render(duration),
		)
	}
}

func inspectJob(host, jobID string, out io.Writer) {
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
	if err != nil {
		fmt.Fprintf(out, "Failed to connect to orchestrator at %s: %v\n", host, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(out, "Failed to fetch job details: %s\n", strings.TrimSpace(string(body)))
		return
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		fmt.Fprintf(out, "Failed to decode response: %v\n", err)
		return
	}

	// Pretty print
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

	fmt.Fprintln(out, titleStyle.Render(fmt.Sprintf("Job Details: %s", job.ID)))
	fmt.Fprintln(out, "")

	printField := func(label, value string) {
		fmt.Fprintf(out, "%s %s\n", labelStyle.Render(label+":"), valueStyle.Render(value))
	}

	printField("Summary", job.Summary)
	printField("Status", job.Status)
	printField("Start Time", job.StartTime.Format(time.RFC3339))
	if !job.EndTime.IsZero() {
		printField("End Time", job.EndTime.Format(time.RFC3339))
		printField("Duration", job.EndTime.Sub(job.StartTime).Round(time.Second).String())
	} else {
		printField("Duration", time.Since(job.StartTime).Round(time.Second).String())
	}

	if job.Error != "" {
		fmt.Fprintf(out, "%s %s\n", labelStyle.Render("Error:"), lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(job.Error))
	}

	fmt.Fprintln(out, "")
	printField("Repo URL", job.WorkItem.RepoURL)

	// Description
	fmt.Fprintln(out, labelStyle.Render("Description:"))
	fmt.Fprintln(out, lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render(job.WorkItem.Description))
	fmt.Fprintln(out, "")

	// Env Vars
	if len(job.WorkItem.EnvVars) > 0 {
		fmt.Fprintln(out, labelStyle.Render("Env Vars:"))
		for k, v := range job.WorkItem.EnvVars {
			if strings.Contains(strings.ToLower(k), "token") || strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "secret") {
				v = "***"
			}
			fmt.Fprintf(out, "  %s=%s\n", k, v)
		}
	}
}

func getLogs(host, jobID string, out io.Writer) {
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s/logs", host, jobID))
	if err != nil {
		fmt.Fprintf(out, "Failed to connect to orchestrator at %s: %v\n", host, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(out, "Failed to fetch logs: %s\n", strings.TrimSpace(string(body)))
		return
	}

	if _, err := io.Copy(out, resp.Body); err != nil {
		fmt.Fprintf(out, "Failed to read logs: %v\n", err)
		return
	}
}

func submitJob(host, file string, wait bool, out io.Writer) {
	f, err := os.Open(file)
	if err != nil {
		fmt.Fprintf(out, "Failed to open file: %v\n", err)
		return
	}
	defer f.Close()

	var item orchestrator.WorkItem
	if err := json.NewDecoder(f).Decode(&item); err != nil {
		fmt.Fprintf(out, "Failed to decode JSON: %v\n", err)
		return
	}

	// Validate ID
	if item.ID == "" {
		fmt.Fprintln(out, "Job ID is required in JSON")
		return
	}

	// Rewind file to send content
	f.Seek(0, 0)

	resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", f)
	if err != nil {
		fmt.Fprintf(out, "Failed to connect to orchestrator at %s: %v\n", host, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(out, "Failed to submit job: %s\n", strings.TrimSpace(string(body)))
		return
	}

	fmt.Fprintf(out, "Job %s submitted successfully.\n", item.ID)

	if wait {
		waitForJob(host, item.ID, out)
	}
}

func waitForJob(host, jobID string, out io.Writer) {
	fmt.Fprintf(out, "Waiting for job %s to complete...\n", jobID)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
		if err != nil {
			fmt.Fprintf(out, "Error polling job status: %v\n", err)
			continue
		}
		defer resp.Body.Close() // In loop, but okay for low frequency

		if resp.StatusCode != http.StatusOK {
			continue
		}

		var job orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
			continue
		}

		if job.Status == "Completed" || job.Status == "Failed" {
			fmt.Fprintf(out, "Job finished with status: %s\n", job.Status)
			getLogs(host, jobID, out)
			return
		}
	}
}

func cancelJob(host, jobID string, out io.Writer) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs/%s", host, jobID), nil)
	if err != nil {
		fmt.Fprintf(out, "Failed to create request: %v\n", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(out, "Failed to connect to orchestrator at %s: %v\n", host, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(out, "Failed to cancel job: %s\n", strings.TrimSpace(string(body)))
		return
	}

	fmt.Fprintf(out, "Job %s cancellation requested.\n", jobID)
}

func retryJob(host, jobID string, out io.Writer) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/%s/retry", host, jobID), nil)
	if err != nil {
		fmt.Fprintf(out, "Failed to create request: %v\n", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(out, "Failed to connect to orchestrator at %s: %v\n", host, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(out, "Failed to retry job: %s\n", strings.TrimSpace(string(body)))
		return
	}

	fmt.Fprintf(out, "Job %s retry submitted successfully.\n", jobID)
}

func retryFailedJobs(host string, out io.Writer) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/retry-failed", host), nil)
	if err != nil {
		fmt.Fprintf(out, "Failed to create request: %v\n", err)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(out, "Failed to connect to orchestrator at %s: %v\n", host, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(out, "Failed to retry failed jobs: %s\n", strings.TrimSpace(string(body)))
		return
	}

	var result struct {
		Retried int `json:"retried"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(out, "Failed to decode response: %v\n", err)
		return
	}

	fmt.Fprintf(out, "Successfully retried %d failed jobs.\n", result.Retried)
}

func limitString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
