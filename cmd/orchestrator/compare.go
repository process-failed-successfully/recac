package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"recac/internal/orchestrator"

	"github.com/charmbracelet/lipgloss"
)

func compareJobs(host, compareJobsIDs string) {
	parts := strings.SplitN(compareJobsIDs, ",", 2)
	if len(parts) != 2 {
		fmt.Fprintf(stdout, "Error: --compare-jobs expects exactly two job IDs separated by a comma (e.g., job1,job2)\n")
		exitFunc(1)
		return
	}

	jobID1 := strings.TrimSpace(parts[0])
	jobID2 := strings.TrimSpace(parts[1])

	if jobID1 == "" || jobID2 == "" {
		fmt.Fprintf(stdout, "Error: Invalid job IDs provided. Format should be job1,job2\n")
		exitFunc(1)
		return
	}

	job1, err := fetchJob(host, jobID1)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to fetch job %s: %v\n", jobID1, err)
		exitFunc(1)
		return
	}

	job2, err := fetchJob(host, jobID2)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to fetch job %s: %v\n", jobID2, err)
		exitFunc(1)
		return
	}

	printJobComparison(job1, job2)
}

func fetchJob(host, jobID string) (*orchestrator.JobInfo, error) {
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
	if err != nil {
		return nil, fmt.Errorf("HTTP GET failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("status %s, body: %s", resp.Status, strings.TrimSpace(string(body)))
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		return nil, fmt.Errorf("JSON decode failed: %w", err)
	}

	return &job, nil
}

func printJobComparison(job1, job2 *orchestrator.JobInfo) {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("#FAFAFA")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("86")).
		Width(20)

	valueStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Width(40)

	diffStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("214")).
		Width(40)

	fmt.Fprintln(stdout, titleStyle.Render(fmt.Sprintf("Job Comparison: %s vs %s", job1.ID, job2.ID)))
	fmt.Fprintln(stdout, "")

	fmt.Fprintf(stdout, "%-20s %-40s %-40s\n", headerStyle.Render("Field"), headerStyle.Render(limitString(job1.ID, 38)), headerStyle.Render(limitString(job2.ID, 38)))
	fmt.Fprintln(stdout, strings.Repeat("-", 102))

	printRow := func(label, val1, val2 string) {
		v1Style := valueStyle
		v2Style := valueStyle
		if val1 != val2 {
			v1Style = diffStyle
			v2Style = diffStyle
		}
		fmt.Fprintf(stdout, "%-20s %-40s %-40s\n", headerStyle.Render(label), v1Style.Render(limitString(val1, 38)), v2Style.Render(limitString(val2, 38)))
	}

	printRow("Status", job1.Status, job2.Status)

	dur1 := time.Since(job1.StartTime).Round(time.Second)
	if job1.Status == "Completed" || job1.Status == "Failed" || job1.Status == "Canceled" {
		if !job1.EndTime.IsZero() {
			dur1 = job1.EndTime.Sub(job1.StartTime).Round(time.Second)
		}
	}
	dur2 := time.Since(job2.StartTime).Round(time.Second)
	if job2.Status == "Completed" || job2.Status == "Failed" || job2.Status == "Canceled" {
		if !job2.EndTime.IsZero() {
			dur2 = job2.EndTime.Sub(job2.StartTime).Round(time.Second)
		}
	}

	dur1Str := dur1.String()
	dur2Str := dur2.String()
	if dur1 != dur2 {
		delta := dur2 - dur1
		deltaStr := fmt.Sprintf("+%v", delta)
		if delta < 0 {
			deltaStr = fmt.Sprintf("%v", delta)
		}
		dur2Str = fmt.Sprintf("%s (%s)", dur2Str, deltaStr)
	}

	printRow("Duration", dur1Str, dur2Str)
	printRow("Agent Provider", job1.WorkItem.AgentProvider, job2.WorkItem.AgentProvider)
	printRow("Agent Model", job1.WorkItem.AgentModel, job2.WorkItem.AgentModel)

	// Compare Environment Variables
	if len(job1.WorkItem.EnvVars) > 0 || len(job2.WorkItem.EnvVars) > 0 {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, headerStyle.Render("Env Vars:"))

		allEnvKeys := make(map[string]bool)
		for k := range job1.WorkItem.EnvVars {
			allEnvKeys[k] = true
		}
		for k := range job2.WorkItem.EnvVars {
			allEnvKeys[k] = true
		}

		for k := range allEnvKeys {
			v1, ok1 := job1.WorkItem.EnvVars[k]
			v2, ok2 := job2.WorkItem.EnvVars[k]
			if !ok1 {
				v1 = "<missing>"
			} else if isSecretEnv(k) {
				v1 = "***"
			}

			if !ok2 {
				v2 = "<missing>"
			} else if isSecretEnv(k) {
				v2 = "***"
			}
			printRow("  "+limitString(k, 18), v1, v2)
		}
	}

	// Compare Metrics
	if len(job1.Metrics) > 0 || len(job2.Metrics) > 0 {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, headerStyle.Render("Metrics:"))

		allMetricKeys := make(map[string]bool)
		for k := range job1.Metrics {
			allMetricKeys[k] = true
		}
		for k := range job2.Metrics {
			allMetricKeys[k] = true
		}

		for k := range allMetricKeys {
			v1, ok1 := job1.Metrics[k]
			v2, ok2 := job2.Metrics[k]

			str1 := "<missing>"
			if ok1 {
				str1 = fmt.Sprintf("%.2f", v1)
			}

			str2 := "<missing>"
			if ok2 {
				delta := v2 - v1
				deltaStr := fmt.Sprintf("+%.2f", delta)
				if delta < 0 {
					deltaStr = fmt.Sprintf("%.2f", delta)
				}
				if !ok1 {
					deltaStr = "new"
				}
				str2 = fmt.Sprintf("%.2f (%s)", v2, deltaStr)
			} else {
				if ok1 {
					str2 = "<missing> (-100%)"
				}
			}
			printRow("  "+limitString(k, 18), str1, str2)
		}
	}

	// Compare Outputs
	if len(job1.Outputs) > 0 || len(job2.Outputs) > 0 {
		fmt.Fprintln(stdout, "")
		fmt.Fprintln(stdout, headerStyle.Render("Outputs:"))

		allOutKeys := make(map[string]bool)
		for k := range job1.Outputs {
			allOutKeys[k] = true
		}
		for k := range job2.Outputs {
			allOutKeys[k] = true
		}

		for k := range allOutKeys {
			v1, ok1 := job1.Outputs[k]
			v2, ok2 := job2.Outputs[k]

			if !ok1 {
				v1 = "<missing>"
			}
			if !ok2 {
				v2 = "<missing>"
			}

			printRow("  "+limitString(k, 18), v1, v2)
		}
	}
	fmt.Fprintln(stdout, "")
}

func isSecretEnv(k string) bool {
	kLower := strings.ToLower(k)
	return strings.Contains(kLower, "token") || strings.Contains(kLower, "key") || strings.Contains(kLower, "secret")
}
