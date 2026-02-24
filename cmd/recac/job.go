package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"recac/internal/orchestrator"
	"recac/internal/tui"
	"text/tabwriter"
	"time"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	jobHost string
)

func init() {
	rootCmd.AddCommand(jobCmd)

	// Persistent flags for job commands
	jobCmd.PersistentFlags().StringVar(&jobHost, "host", "http://localhost:2112", "Orchestrator host URL")
	viper.BindPFlag("orchestrator.host", jobCmd.PersistentFlags().Lookup("host"))
	viper.BindEnv("orchestrator.host", "RECAC_ORCHESTRATOR_HOST")

	// Subcommands
	jobCmd.AddCommand(jobListCmd)
	jobCmd.AddCommand(jobMonitorCmd)
	jobCmd.AddCommand(jobInfoCmd)
	jobCmd.AddCommand(jobLogsCmd)
	jobCmd.AddCommand(jobSubmitCmd)
	jobCmd.AddCommand(jobCancelCmd)
	jobCmd.AddCommand(jobRetryCmd)
	jobCmd.AddCommand(jobRetryFailedCmd)

	// List flags
	jobListCmd.Flags().Bool("all", false, "Show all jobs (including completed)")

	// Submit flags
	jobSubmitCmd.Flags().String("id", "", "Optional job ID")
	jobSubmitCmd.Flags().Bool("wait", false, "Wait for job completion")
}

var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage orchestrator jobs",
	Long:  `List, monitor, submit, and manage jobs on the RECAC Orchestrator.`,
}

var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active jobs",
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("orchestrator.host")
		all, _ := cmd.Flags().GetBool("all")

		url := fmt.Sprintf("%s/jobs", host)
		if all {
			url += "?state=all"
		}

		resp, err := http.Get(url)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to connect to orchestrator at %s: %v\n", host, err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to fetch jobs: status %s\n", resp.Status)
			os.Exit(1)
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to decode response: %v\n", err)
			os.Exit(1)
		}

		if len(jobs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No jobs found.")
			return
		}

		w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 3, ' ', 0)
		fmt.Fprintln(w, "ID\tSUMMARY\tSTATUS\tDURATION")
		for _, job := range jobs {
			duration := time.Since(job.StartTime).Round(time.Second).String()
			if !job.EndTime.IsZero() {
				duration = job.EndTime.Sub(job.StartTime).Round(time.Second).String()
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				job.ID,
				limitString(job.Summary, 50),
				job.Status,
				duration,
			)
		}
		w.Flush()
	},
}

var jobMonitorCmd = &cobra.Command{
	Use:   "monitor",
	Short: "Launch TUI dashboard",
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("orchestrator.host")
		if err := tui.StartDashboard(host); err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Dashboard failed: %v\n", err)
			os.Exit(1)
		}
	},
}

var jobInfoCmd = &cobra.Command{
	Use:   "info [job-id]",
	Short: "Show job details",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("orchestrator.host")
		jobID := args[0]

		resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to connect: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(cmd.OutOrStderr(), "Job not found or error: %s\n", resp.Status)
			os.Exit(1)
		}

		var job orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to decode: %v\n", err)
			os.Exit(1)
		}

		// Use lipgloss for consistent style
		titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86"))

		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", titleStyle.Render("ID"), job.ID)
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", titleStyle.Render("Summary"), job.Summary)
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", titleStyle.Render("Status"), job.Status)
		fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", titleStyle.Render("Repo"), job.WorkItem.RepoURL)

		if job.Error != "" {
			errStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
			fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", titleStyle.Render("Error"), errStyle.Render(job.Error))
		}

		fmt.Fprintln(cmd.OutOrStdout(), "")
		fmt.Fprintln(cmd.OutOrStdout(), job.WorkItem.Description)
	},
}

var jobLogsCmd = &cobra.Command{
	Use:   "logs [job-id]",
	Short: "Stream job logs",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("orchestrator.host")
		jobID := args[0]

		resp, err := http.Get(fmt.Sprintf("%s/jobs/%s/logs", host, jobID))
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to connect: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to fetch logs: %s\n%s\n", resp.Status, string(body))
			os.Exit(1)
		}

		io.Copy(cmd.OutOrStdout(), resp.Body)
	},
}

var jobSubmitCmd = &cobra.Command{
	Use:   "submit [repo-url] [task-description]",
	Short: "Submit a new job",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("orchestrator.host")
		repoURL := args[0]
		task := args[1]
		id, _ := cmd.Flags().GetString("id")
		wait, _ := cmd.Flags().GetBool("wait")

		if id == "" {
			// generate random ID if not provided
			id = fmt.Sprintf("job-%d", time.Now().Unix())
		}

		item := orchestrator.WorkItem{
			ID:          id,
			Summary:     task, // Use task as summary
			Description: task,
			RepoURL:     repoURL,
		}

		body, _ := json.Marshal(item)
		resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", bytes.NewReader(body))
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to submit job: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted {
			respBody, _ := io.ReadAll(resp.Body)
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to submit job: %s\n%s\n", resp.Status, string(respBody))
			os.Exit(1)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Job %s submitted.\n", id)

		if wait {
			fmt.Fprintln(cmd.OutOrStdout(), "Waiting for completion...")
			// TODO: Poll status or stream logs
			// For now, just stream logs
			time.Sleep(2 * time.Second) // Give it a sec to spawn

			logResp, err := http.Get(fmt.Sprintf("%s/jobs/%s/logs", host, id))
			if err == nil && logResp.StatusCode == http.StatusOK {
				defer logResp.Body.Close()
				io.Copy(cmd.OutOrStdout(), logResp.Body)
			}
		}
	},
}

var jobCancelCmd = &cobra.Command{
	Use:   "cancel [job-id]",
	Short: "Cancel a running job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("orchestrator.host")
		jobID := args[0]

		req, _ := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs/%s", host, jobID), nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to cancel job: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to cancel job: %s\n", resp.Status)
			os.Exit(1)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Job %s cancelled.\n", jobID)
	},
}

var jobRetryCmd = &cobra.Command{
	Use:   "retry [job-id]",
	Short: "Retry a completed job",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("orchestrator.host")
		jobID := args[0]

		resp, err := http.Post(fmt.Sprintf("%s/jobs/%s/retry", host, jobID), "application/json", nil)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to retry job: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to retry job: %s\n", resp.Status)
			os.Exit(1)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Job %s retry submitted.\n", jobID)
	},
}

var jobRetryFailedCmd = &cobra.Command{
	Use:   "retry-failed",
	Short: "Retry all failed jobs",
	Run: func(cmd *cobra.Command, args []string) {
		host := viper.GetString("orchestrator.host")

		resp, err := http.Post(fmt.Sprintf("%s/jobs/retry-failed", host), "application/json", nil)
		if err != nil {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to retry jobs: %v\n", err)
			os.Exit(1)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Fprintf(cmd.OutOrStderr(), "Failed to retry jobs: %s\n", resp.Status)
			os.Exit(1)
		}

		var result struct {
			Retried int `json:"retried"`
		}
		json.NewDecoder(resp.Body).Decode(&result)
		fmt.Fprintf(cmd.OutOrStdout(), "Retrying %d failed jobs.\n", result.Retried)
	},
}

func limitString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
