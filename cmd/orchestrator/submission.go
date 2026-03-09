package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"recac/internal/orchestrator"

	"github.com/google/uuid"
)

var (
	exitFunc           = os.Exit
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

func submitMatrixJob(host, filePath string, wait bool) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	resp, err := http.Post(fmt.Sprintf("%s/jobs/matrix", host), "application/json", file)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit matrix job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Submitted []string `json:"submitted"`
		Errors    []string `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to parse matrix response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Matrix submission completed.\n")
	if len(result.Submitted) > 0 {
		fmt.Fprintf(stdout, "Successfully submitted jobs: %s\n", strings.Join(result.Submitted, ", "))
	}
	if len(result.Errors) > 0 {
		fmt.Fprintf(stdout, "Errors:\n")
		for _, e := range result.Errors {
			fmt.Fprintf(stdout, "  - %s\n", e)
		}
		if len(result.Submitted) == 0 {
			exitFunc(1)
			return
		}
	}

	if wait && len(result.Submitted) > 0 {
		for _, id := range result.Submitted {
			if err := waitForJob(host, id, stdout); err != nil {
				fmt.Fprintf(stdout, "Job %s failed: %v\n", id, err)
				exitFunc(1)
				return
			}
		}
	}
}

func submitBatchJob(host, filePath string, wait bool) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	resp, err := http.Post(fmt.Sprintf("%s/jobs/batch", host), "application/json", file)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit batch job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Submitted []string `json:"submitted"`
		Errors    []string `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to parse batch response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Batch submission completed.\n")
	if len(result.Submitted) > 0 {
		fmt.Fprintf(stdout, "Successfully submitted jobs: %s\n", strings.Join(result.Submitted, ", "))
	}
	if len(result.Errors) > 0 {
		fmt.Fprintf(stdout, "Errors:\n")
		for _, e := range result.Errors {
			fmt.Fprintf(stdout, "  - %s\n", e)
		}
		// If there were any errors, exit with non-zero
		if len(result.Submitted) == 0 {
			exitFunc(1)
			return
		}
	}

	if wait && len(result.Submitted) > 0 {
		for _, id := range result.Submitted {
			if err := waitForJob(host, id, stdout); err != nil {
				fmt.Fprintf(stdout, "Job %s failed: %v\n", id, err)
				exitFunc(1)
				return
			}
		}
	}
}

func cloneJob(host, originalID, newID string, priority *int, wait bool, envVars map[string]string, dependsOn []string) {
	overrides := struct {
		NewID     string            `json:"new_id,omitempty"`
		EnvVars   map[string]string `json:"env_vars,omitempty"`
		Priority  *int              `json:"priority,omitempty"`
		DependsOn []string          `json:"depends_on,omitempty"`
	}{
		NewID:     newID,
		EnvVars:   envVars,
		Priority:  priority,
		DependsOn: dependsOn,
	}

	payload, err := json.Marshal(overrides)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal overrides: %v\n", err)
		exitFunc(1)
		return
	}

	cloneURL := fmt.Sprintf("%s/jobs/%s/clone", host, url.PathEscape(originalID))
	resp, err := http.Post(cloneURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to clone job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		ClonedJobID string `json:"cloned_job_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to parse response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s cloned successfully as %s\n", originalID, result.ClonedJobID)

	if wait {
		if err := waitForJob(host, result.ClonedJobID, stdout); err != nil {
			fmt.Fprintf(stdout, "Job failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func submitAdHocJob(host, repo, task, id string, priority int, delay time.Duration, wait bool, envVars map[string]string, dependsOn []string, tags []string) {
	if id == "" {
		id = uuid.New().String()
	}

	item := orchestrator.WorkItem{
		ID:          id,
		Summary:     task, // Using task description as summary for ad-hoc
		Description: task,
		RepoURL:     repo,
		EnvVars:     envVars,
		Priority:    priority,
		DependsOn:   dependsOn,
		Tags:        tags,
	}

	if delay > 0 {
		item.RunAfter = time.Now().Add(delay)
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
		if job.Status == "Canceled" {
			return fmt.Errorf("job canceled with error: %s", job.Error)
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
						if finalJob.Status == "Canceled" {
							return fmt.Errorf("job canceled with error: %s", finalJob.Error)
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

func clearPending(host string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/pending", host), nil)
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
		fmt.Fprintf(stdout, "Failed to clear pending jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	cleared, ok := result["cleared"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully cleared %d jobs from pending queue.\n", int(cleared))
}

func clearHistory(host string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/history", host), nil)
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
		fmt.Fprintf(stdout, "Failed to clear history: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	cleared, ok := result["cleared"].(float64)
	if !ok {
		fmt.Fprintf(stdout, "Unexpected response format\n")
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully cleared %d jobs from history.\n", int(cleared))
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

func cancelJobsByTag(host, tag string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs?tag=%s", host, url.QueryEscape(tag)), nil)
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
		fmt.Fprintf(stdout, "Failed to cancel jobs by tag: %s\n", strings.TrimSpace(string(body)))
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

	fmt.Fprintf(stdout, "Successfully canceled %d jobs with tag '%s'.\n", int(canceled), tag)
}

func updatePriority(host, jobID string, priority int) {
	urlStr := fmt.Sprintf("%s/jobs/%s/priority", host, jobID)
	reqBody := fmt.Sprintf(`{"priority": %d}`, priority)

	req, err := http.NewRequest(http.MethodPut, urlStr, strings.NewReader(reqBody))
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
		fmt.Fprintf(stdout, "Failed to update priority: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s priority updated to %d\n", jobID, priority)
}

func exportJobs(host, path, format string) {
	if format != "json" && format != "csv" {
		fmt.Fprintf(stdout, "Invalid format: %s. Must be 'json' or 'csv'.\n", format)
		exitFunc(1)
		return
	}

	url := fmt.Sprintf("%s/jobs/export?format=%s", host, format)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to export jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	if path == "-" {
		// Output to stdout
		io.Copy(stdout, resp.Body)
	} else {
		// Output to file
		file, err := os.Create(path)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to create output file %s: %v\n", path, err)
			exitFunc(1)
			return
		}
		defer file.Close()

		_, err = io.Copy(file, resp.Body)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to write to file %s: %v\n", path, err)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "Jobs successfully exported to %s\n", path)
	}
}
