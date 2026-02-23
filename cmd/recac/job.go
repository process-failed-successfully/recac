package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"recac/internal/orchestrator"
	"recac/internal/tui"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// startDashboardFunc allows mocking the TUI dashboard in tests.
var startDashboardFunc = tui.StartDashboard

func init() {
	rootCmd.AddCommand(NewJobCmd())
}

func NewJobCmd() *cobra.Command {
	jobCmd := &cobra.Command{
		Use:   "job",
		Short: "Manage orchestrator jobs",
		Long:  `Manage and monitor orchestrator jobs (list, logs, monitor, submit, cancel).`,
	}

	jobCmd.PersistentFlags().String("host", "http://localhost:2112", "Orchestrator host URL")
	viper.BindPFlag("orchestrator.host", jobCmd.PersistentFlags().Lookup("host"))

	jobCmd.AddCommand(newJobListCmd())
	jobCmd.AddCommand(newJobMonitorCmd())
	jobCmd.AddCommand(newJobLogsCmd())
	jobCmd.AddCommand(newJobInfoCmd())
	jobCmd.AddCommand(newJobCancelCmd())
	jobCmd.AddCommand(newJobSubmitCmd())

	return jobCmd
}

func newJobListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List active jobs",
		Long:  `List active jobs running on the orchestrator. Use --all to include completed jobs.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			host := getOrchestratorHost()
			all, _ := cmd.Flags().GetBool("all")

			url := fmt.Sprintf("%s/jobs", host)
			if all {
				url += "?state=all"
			}

			resp, err := http.Get(url)
			if err != nil {
				return fmt.Errorf("failed to connect to orchestrator at %s: %w", host, err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("orchestrator returned status %s", resp.Status)
			}

			var jobs []orchestrator.JobInfo
			if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}

			if len(jobs) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "No jobs found.")
				return nil
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
					limitString(job.Summary, 40),
					job.Status,
					duration,
				)
			}
			w.Flush()
			return nil
		},
	}
	cmd.Flags().Bool("all", false, "Include completed jobs")
	return cmd
}

func newJobMonitorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "monitor",
		Short: "Launch the TUI dashboard",
		Long:  `Launch the interactive TUI dashboard to monitor the orchestrator.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			host := getOrchestratorHost()
			if err := startDashboardFunc(host); err != nil {
				return fmt.Errorf("dashboard failed: %w", err)
			}
			return nil
		},
	}
}

func newJobLogsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logs [job-id]",
		Short: "Stream logs for a job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := getOrchestratorHost()
			jobID := args[0]

			resp, err := http.Get(fmt.Sprintf("%s/jobs/%s/logs", host, jobID))
			if err != nil {
				return fmt.Errorf("failed to connect to orchestrator: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to fetch logs (status %s)", resp.Status)
			}

			if _, err := io.Copy(cmd.OutOrStdout(), resp.Body); err != nil {
				return fmt.Errorf("failed to stream logs: %w", err)
			}
			return nil
		},
	}
}

func newJobInfoCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "info [job-id]",
		Short: "Show job details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := getOrchestratorHost()
			jobID := args[0]

			resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
			if err != nil {
				return fmt.Errorf("failed to connect to orchestrator: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to fetch job details (status %s)", resp.Status)
			}

			var job orchestrator.JobInfo
			if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
				return fmt.Errorf("failed to decode response: %w", err)
			}

			// Pretty print JSON
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			if err := enc.Encode(job); err != nil {
				return fmt.Errorf("failed to encode output: %w", err)
			}
			return nil
		},
	}
}

func newJobCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel [job-id]",
		Short: "Cancel a running job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			host := getOrchestratorHost()
			jobID := args[0]

			req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs/%s", host, jobID), nil)
			if err != nil {
				return fmt.Errorf("failed to create request: %w", err)
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				return fmt.Errorf("failed to connect to orchestrator: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to cancel job (status %s)", resp.Status)
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Job %s cancellation requested.\n", jobID)
			return nil
		},
	}
}

func newJobSubmitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit",
		Short: "Submit a new job",
		Long:  `Submit a new job to the orchestrator. Requires --id, --summary, --repo-url.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			host := getOrchestratorHost()

			// Parse flags
			id, _ := cmd.Flags().GetString("id")
			summary, _ := cmd.Flags().GetString("summary")
			desc, _ := cmd.Flags().GetString("description")
			repoURL, _ := cmd.Flags().GetString("repo-url")

			if id == "" || summary == "" || repoURL == "" {
				return fmt.Errorf("--id, --summary, and --repo-url are required")
			}

			item := orchestrator.WorkItem{
				ID:          id,
				Summary:     summary,
				Description: desc,
				RepoURL:     repoURL,
				EnvVars:     make(map[string]string),
			}

			// Optional env vars parsing could be added here

			body, err := json.Marshal(item)
			if err != nil {
				return fmt.Errorf("failed to marshal request body: %w", err)
			}

			resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", bytes.NewBuffer(body))
			if err != nil {
				return fmt.Errorf("failed to connect to orchestrator: %w", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
				msg, _ := io.ReadAll(resp.Body)
				return fmt.Errorf("failed to submit job (status %s): %s", resp.Status, string(msg))
			}

			fmt.Fprintf(cmd.OutOrStdout(), "Job %s submitted successfully.\n", id)
			return nil
		},
	}
	cmd.Flags().String("id", "", "Job ID")
	cmd.Flags().String("summary", "", "Job Summary")
	cmd.Flags().String("description", "", "Job Description")
	cmd.Flags().String("repo-url", "", "Repository URL")
	return cmd
}

func getOrchestratorHost() string {
	// Check flag first (via viper binding), then config, then env
	host := viper.GetString("orchestrator.host")
	if host == "" {
		host = "http://localhost:2112"
	}
	// Ensure no trailing slash
	return strings.TrimRight(host, "/")
}

func limitString(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}
