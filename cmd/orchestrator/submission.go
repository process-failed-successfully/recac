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

	"recac/internal/orchestrator"

	"github.com/google/uuid"
)

var (
	exitFunc = os.Exit
	stdout   io.Writer = os.Stdout
)

func submitJob(host, filePath string, wait bool) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	// Verify JSON validity before sending (optional but good UX)
	var item map[string]interface{}
	if err := json.NewDecoder(file).Decode(&item); err != nil {
		fmt.Fprintf(stdout, "Invalid JSON in file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	// Extract ID for waiting
	id, _ := item["id"].(string)

	// Reset file pointer
	file.Seek(0, 0)

	resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", file)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "%s\n", strings.TrimSpace(string(body)))

	if wait && id != "" {
		if err := waitForJob(host, id, stdout); err != nil {
			fmt.Fprintf(stdout, "Job failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func submitAdHocJob(host, repo, task, id string, wait bool, envVars map[string]string) {
	if id == "" {
		id = uuid.New().String()
	}

	item := orchestrator.WorkItem{
		ID:          id,
		Summary:     task, // Using task description as summary for ad-hoc
		Description: task,
		RepoURL:     repo,
		EnvVars:     envVars,
	}

	payload, err := json.Marshal(item)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal work item: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "%s\n", strings.TrimSpace(string(body)))

	if wait {
		if err := waitForJob(host, id, stdout); err != nil {
			fmt.Fprintf(stdout, "Job failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func waitForJob(host, jobID string, out io.Writer) error {
	fmt.Fprintf(out, "Waiting for job %s to start...\n", jobID)

	// Poll until active or completed
	for {
		resp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
		if err != nil {
			// Retry on network error
			time.Sleep(1 * time.Second)
			continue
		}

		var job orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
			resp.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}
		resp.Body.Close()

		if job.Status == "Completed" {
			fmt.Fprintln(out, "Job already completed.")
			return nil
		}
		if job.Status == "Failed" {
			return fmt.Errorf("job failed with error: %s", job.Error)
		}

		// If job is in a state where it might have logs (Running, Spawning which often implies running in Docker)
		// We try to stream logs.
		// Note: "Spawning" is the status during execution in DockerSpawner.
		if job.Status == "Spawning" || job.Status == "Running" || job.Status == "Active" {
			// Try to stream logs
			logsResp, err := http.Get(fmt.Sprintf("%s/jobs/%s/logs", host, jobID))
			if err == nil && logsResp.StatusCode == http.StatusOK {
				fmt.Fprintln(out, "--- Log Stream Start ---")
				io.Copy(out, logsResp.Body)
				logsResp.Body.Close()
				fmt.Fprintln(out, "\n--- Log Stream End ---")

				// Logs finished, check final status
				finalResp, err := http.Get(fmt.Sprintf("%s/jobs/%s", host, jobID))
				if err == nil {
					var finalJob orchestrator.JobInfo
					if err := json.NewDecoder(finalResp.Body).Decode(&finalJob); err == nil {
						if finalJob.Status == "Failed" {
							return fmt.Errorf("job failed with error: %s", finalJob.Error)
						}
						// If logs finished, assume success unless failed?
						// But if status is still Spawning, it might be weird.
						// However, if streaming ends, container stopped.
						// Orchestrator moves to Completed/Failed AFTER container stops.
						// So there might be a tiny race where status is still Spawning.
						// We should probably loop again if status is still Spawning?
						if finalJob.Status == "Completed" {
							return nil
						}
					}
					finalResp.Body.Close()
				}
				// Continue loop to check status again
				continue
			} else {
				if logsResp != nil {
					logsResp.Body.Close()
				}
				// Maybe container not ready yet
				time.Sleep(1 * time.Second)
				continue
			}
		}

		time.Sleep(1 * time.Second)
	}
}

func cancelAllJobs(host string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs", host), nil)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to cancel jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	canceled, ok := result["canceled"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully canceled %d jobs.\n", int(canceled))
}
