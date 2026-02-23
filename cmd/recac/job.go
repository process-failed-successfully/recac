package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"recac/internal/orchestrator"
	"github.com/google/uuid"
	"recac/internal/tui"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

var jobHost string

// jobCmd represents the job command
var jobCmd = &cobra.Command{
	Use:   "job",
	Short: "Manage orchestrator jobs",
	Long:  `Manage and monitor jobs running on the RECAC orchestrator.`,
}

var jobMonitorCmd = &cobra.Command{
	Use:     "monitor",
	Aliases: []string{"dashboard"},
	Short:   "Launch the TUI dashboard",
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.StartDashboard(jobHost)
	},
}

var jobLogsCmd = &cobra.Command{
	Use:   "logs [ID]",
	Short: "Get logs for a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		resp, err := doRequest("GET", fmt.Sprintf("%s/jobs/%s/logs", jobHost, id), nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status: %s", resp.Status)
		}

		_, err = io.Copy(cmd.OutOrStdout(), resp.Body)
		return err
	},
}

var jobCancelCmd = &cobra.Command{
	Use:   "cancel [ID]",
	Short: "Cancel a job",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		resp, err := doRequest("DELETE", fmt.Sprintf("%s/jobs/%s", jobHost, id), nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status: %s", resp.Status)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Job %s cancelled.\n", id)
		return nil
	},
}

var jobSubmitCmd = &cobra.Command{
	Use:   "submit",
	Short: "Submit a new job",
	RunE: func(cmd *cobra.Command, args []string) error {
		repo, _ := cmd.Flags().GetString("repo")
		task, _ := cmd.Flags().GetString("task")
		id, _ := cmd.Flags().GetString("id")

		if task == "" {
			return fmt.Errorf("task description is required")
		}

		if id == "" {
			id = fmt.Sprintf("JOB-%s", uuid.New().String()[:8])
		}

		item := orchestrator.WorkItem{
			ID:          id,
			Summary:     task, // Using task as summary for now
			Description: task,
			RepoURL:     repo,
			EnvVars:     make(map[string]string),
		}

		body, err := json.Marshal(item)
		if err != nil {
			return err
		}

		resp, err := doRequest("POST", fmt.Sprintf("%s/jobs", jobHost), bytes.NewReader(body))
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusAccepted {
			b, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("server returned status: %s (%s)", resp.Status, string(b))
		}

		fmt.Fprintf(cmd.OutOrStdout(), "Job submitted successfully: %s\n", id)
		return nil
	},
}

var jobInfoCmd = &cobra.Command{
	Use:     "info [ID]",
	Aliases: []string{"inspect"},
	Short:   "Show detailed job information",
	Args:    cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		resp, err := doRequest("GET", fmt.Sprintf("%s/jobs/%s", jobHost, id), nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status: %s", resp.Status)
		}

		var job orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		// Rendering
		h1 := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Width(15).Render
		val := lipgloss.NewStyle().Foreground(lipgloss.Color("252")).Render

		out := cmd.OutOrStdout()
		fmt.Fprintf(out, "%s %s\n", h1("ID:"), val(job.ID))
		fmt.Fprintf(out, "%s %s\n", h1("Status:"), val(job.Status))
		fmt.Fprintf(out, "%s %s\n", h1("Started:"), val(job.StartTime.Format(time.RFC3339)))
		if !job.EndTime.IsZero() {
			fmt.Fprintf(out, "%s %s\n", h1("Ended:"), val(job.EndTime.Format(time.RFC3339)))
		}
		if job.Error != "" {
			fmt.Fprintf(out, "%s %s\n", h1("Error:"), lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Render(job.Error))
		}

		fmt.Fprintln(out)
		fmt.Fprintf(out, "%s %s\n", h1("Repo:"), val(job.WorkItem.RepoURL))
		fmt.Fprintf(out, "%s %s\n", h1("Summary:"), val(job.Summary))
		fmt.Fprintf(out, "%s\n%s\n", h1("Description:"), val(job.WorkItem.Description))

		if len(job.WorkItem.EnvVars) > 0 {
			fmt.Fprintln(out)
			fmt.Fprintln(out, h1("Env Vars:"))
			for k, v := range job.WorkItem.EnvVars {
				if strings.Contains(strings.ToLower(k), "token") || strings.Contains(strings.ToLower(k), "key") {
					v = "***"
				}
				fmt.Fprintf(out, "  %s=%s\n", k, v)
			}
		}

		return nil
	},
}

var jobListCmd = &cobra.Command{
	Use:   "list",
	Short: "List active and completed jobs",
	RunE: func(cmd *cobra.Command, args []string) error {
		resp, err := doRequest("GET", fmt.Sprintf("%s/jobs?state=all", jobHost), nil)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("server returned status: %s", resp.Status)
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}

		if len(jobs) == 0 {
			fmt.Fprintln(cmd.OutOrStdout(), "No jobs found.")
			return nil
		}

		// Rendering
		headerStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("252")).Padding(0, 1)
		rowStyle := lipgloss.NewStyle().Padding(0, 1)

		fmt.Fprintf(cmd.OutOrStdout(), "%-15s %-40s %-15s %-20s\n",
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
			summary := job.Summary
			if len(summary) > 38 {
				summary = summary[:35] + "..."
			}

			fmt.Fprintf(cmd.OutOrStdout(), "%-15s %-40s %-15s %-20s\n",
				rowStyle.Render(job.ID),
				rowStyle.Render(summary),
				rowStyle.Render(job.Status),
				rowStyle.Render(duration),
			)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(jobCmd)
	jobCmd.PersistentFlags().StringVar(&jobHost, "host", "http://localhost:2112", "Orchestrator host URL")
	jobCmd.AddCommand(jobListCmd)
	jobCmd.AddCommand(jobMonitorCmd)
	jobCmd.AddCommand(jobInfoCmd)
	jobCmd.AddCommand(jobLogsCmd)
	jobCmd.AddCommand(jobSubmitCmd)
	jobCmd.AddCommand(jobCancelCmd)

	jobSubmitCmd.Flags().String("repo", "", "Repository URL")
	jobSubmitCmd.Flags().String("task", "", "Task description")
	jobSubmitCmd.Flags().String("id", "", "Optional Job ID")
}

// Helper to make HTTP requests
func doRequest(method, url string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to orchestrator: %w", err)
	}
	return resp, nil
}
