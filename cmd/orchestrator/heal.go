package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"recac/internal/orchestrator"
)

func healJob(host, jobID string, wait bool) {
	fmt.Fprintf(stdout, "Fetching job details for %s...\n", jobID)

	// 1. Fetch Job Metadata
	resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch job details: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// 2. Fetch Job Logs
	fmt.Fprintf(stdout, "Fetching logs for %s...\n", jobID)
	logResp, err := http.Get(fmt.Sprintf("%s/jobs/%s/logs", host, jobID))
	var logsText string
	if err != nil {
		fmt.Fprintf(stdout, "Warning: Failed to fetch logs: %v\n", err)
	} else {
		defer logResp.Body.Close()
		if logResp.StatusCode == http.StatusOK {
			logBytes, _ := io.ReadAll(logResp.Body)
			logsText = string(logBytes)
		} else {
			fmt.Fprintf(stdout, "Warning: Failed to fetch logs, status %d\n", logResp.StatusCode)
		}
	}

	// Extract the last 500 lines of logs to avoid context overflow
	logLines := strings.Split(logsText, "\n")
	if len(logLines) > 500 {
		logLines = logLines[len(logLines)-500:]
		logsText = "... [Logs Truncated] ...\n" + strings.Join(logLines, "\n")
	}

	// 3. Construct new WorkItem
	newItem := job.WorkItem
	newItem.ID = fmt.Sprintf("%s-healed", job.ID)

	// Embed failure context into the description
	failureContext := fmt.Sprintf("\n\n---\nPrevious Job Failure Context:\nError: %s\nLogs:\n```\n%s\n```\n", job.Error, logsText)
	newItem.Description = newItem.Description + failureContext

	// Append auto-heal tag
	hasAutoHealTag := false
	for _, tag := range newItem.Tags {
		if tag == "auto-heal" {
			hasAutoHealTag = true
			break
		}
	}
	if !hasAutoHealTag {
		newItem.Tags = append(newItem.Tags, "auto-heal")
	}

	// 4. Resubmit the job
	fmt.Fprintf(stdout, "Submitting healed job %s...\n", newItem.ID)
	payload, err := json.Marshal(newItem)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal healed job: %v\n", err)
		exitFunc(1)
		return
	}

	postResp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer postResp.Body.Close()

	body, _ := io.ReadAll(postResp.Body)
	if postResp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit healed job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Healed job %s submitted successfully.\n", newItem.ID)

	if wait {
		if err := waitForJob(host, newItem.ID, stdout); err != nil {
			fmt.Fprintf(stdout, "Healed job failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}
