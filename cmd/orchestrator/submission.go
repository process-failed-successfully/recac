package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
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

func lintPipelineJob(filePath string) {
	fileData, err := os.ReadFile(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}

	items, err := orchestrator.ParsePipelineToWorkItems(fileData)
	if err != nil {
		fmt.Fprintf(stdout, "Pipeline validation failed: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Pipeline is valid. Parsed %d jobs.\n", len(items))
}

func submitPipelineJob(host, filePath string, wait bool, dryRun bool) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", filePath, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	if dryRun {
		fileData, err := io.ReadAll(file)
		if err != nil {
			fmt.Fprintf(stdout, "Failed to read file %s: %v\n", filePath, err)
			exitFunc(1)
			return
		}

		items, err := orchestrator.ParsePipelineToWorkItems(fileData)
		if err != nil {
			fmt.Fprintf(stdout, "Pipeline validation failed: %v\n", err)
			exitFunc(1)
			return
		}

		fmt.Fprintf(stdout, "Pipeline valid. Dry run generated %d jobs:\n", len(items))
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(items); err != nil {
			fmt.Fprintf(stdout, "Failed to encode items to JSON: %v\n", err)
			exitFunc(1)
			return
		}
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/jobs/pipeline", host), "application/x-yaml", file)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit pipeline job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Submitted []string `json:"submitted"`
		Errors    []string `json:"errors"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to parse pipeline response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Pipeline submission completed.\n")
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

func submitAdHocJob(host, repo, task, id string, priority int, delay, timeout time.Duration, maxRetries *int, wait bool, envVars map[string]string, dependsOn []string, tags []string, concurrencyGroup string, cancelInProgress bool, agentProvider string, agentModel string) {
	if id == "" {
		id = uuid.New().String()
	}

	item := orchestrator.WorkItem{
		ID:               id,
		Summary:          task, // Using task description as summary for ad-hoc
		Description:      task,
		RepoURL:          repo,
		EnvVars:          envVars,
		Priority:         priority,
		DependsOn:        dependsOn,
		Tags:             tags,
		Delay:            delay,
		Timeout:          timeout,
		ConcurrencyGroup: concurrencyGroup,
		CancelInProgress: cancelInProgress,
		AgentProvider:    agentProvider,
		AgentModel:       agentModel,
		MaxRetries:       maxRetries,
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

func waitForTag(host, tag string, out io.Writer) error {
	fmt.Fprintf(out, "Waiting for jobs with tag '%s' to complete...\n", tag)

	urlStr := fmt.Sprintf("%s/jobs?state=all&tag=%s", host, url.QueryEscape(tag))

	for {
		resp, err := http.Get(urlStr)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			resp.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}
		resp.Body.Close()

		if len(jobs) == 0 {
			fmt.Fprintf(out, "No jobs found with tag '%s'.\n", tag)
			return nil
		}

		allCompleted := true
		for _, job := range jobs {
			if job.Status == "Failed" {
				return fmt.Errorf("job %s failed with error: %s", job.ID, job.Error)
			}
			if job.Status == "Canceled" {
				return fmt.Errorf("job %s canceled with error: %s", job.ID, job.Error)
			}
			if job.Status != "Completed" {
				allCompleted = false
			}
		}

		if allCompleted {
			fmt.Fprintf(out, "All jobs with tag '%s' completed successfully.\n", tag)
			return nil
		}

		time.Sleep(1 * time.Second)
	}
}

func waitForMatch(host, match string, out io.Writer) error {
	fmt.Fprintf(out, "Waiting for jobs matching '%s' to complete...\n", match)

	urlStr := fmt.Sprintf("%s/jobs?state=all&match=%s", host, url.QueryEscape(match))

	for {
		resp, err := http.Get(urlStr)
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		var jobs []orchestrator.JobInfo
		if err := json.NewDecoder(resp.Body).Decode(&jobs); err != nil {
			resp.Body.Close()
			time.Sleep(1 * time.Second)
			continue
		}
		resp.Body.Close()

		if len(jobs) == 0 {
			fmt.Fprintf(out, "No jobs found matching '%s'.\n", match)
			return nil
		}

		allCompleted := true
		for _, job := range jobs {
			if job.Status == "Failed" {
				return fmt.Errorf("job %s failed with error: %s", job.ID, job.Error)
			}
			if job.Status == "Canceled" {
				return fmt.Errorf("job %s canceled with error: %s", job.ID, job.Error)
			}
			if job.Status != "Completed" {
				allCompleted = false
			}
		}

		if allCompleted {
			fmt.Fprintf(out, "All jobs matching '%s' completed successfully.\n", match)
			return nil
		}

		time.Sleep(1 * time.Second)
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

func cancelJobsByStatus(host, status string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs?status=%s", host, url.QueryEscape(status)), nil)
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
		fmt.Fprintf(stdout, "Failed to cancel jobs by status: %s\n", strings.TrimSpace(string(body)))
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

	fmt.Fprintf(stdout, "Successfully canceled %d jobs with status '%s'.\n", int(canceled), status)
}

func cancelJobsByMatch(host, match string) {
	req, err := http.NewRequest(http.MethodDelete, fmt.Sprintf("%s/jobs?match=%s", host, url.QueryEscape(match)), nil)
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
		fmt.Fprintf(stdout, "Failed to cancel jobs by match: %s\n", strings.TrimSpace(string(body)))
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

	fmt.Fprintf(stdout, "Successfully canceled %d jobs matching '%s'.\n", int(canceled), match)
}

func purgeJobsByTag(host, tag string) {
	urlStr := fmt.Sprintf("%s/history?tag=%s", host, url.QueryEscape(tag))
	req, err := http.NewRequest(http.MethodDelete, urlStr, nil)
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
		fmt.Fprintf(stdout, "Failed to purge jobs by tag: %s\n", strings.TrimSpace(string(body)))
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

	fmt.Fprintf(stdout, "Successfully purged %d jobs with tag '%s'.\n", int(cleared), tag)
}

func purgeJobsByStatus(host, status string) {
	urlStr := fmt.Sprintf("%s/history?status=%s", host, url.QueryEscape(status))
	req, err := http.NewRequest(http.MethodDelete, urlStr, nil)
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
		fmt.Fprintf(stdout, "Failed to purge jobs by status: %s\n", strings.TrimSpace(string(body)))
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

	fmt.Fprintf(stdout, "Successfully purged %d jobs with status '%s'.\n", int(cleared), status)
}

func purgeJobsByMatch(host, match string) {
	urlStr := fmt.Sprintf("%s/history?match=%s", host, url.QueryEscape(match))
	req, err := http.NewRequest(http.MethodDelete, urlStr, nil)
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
		fmt.Fprintf(stdout, "Failed to purge jobs by match: %s\n", strings.TrimSpace(string(body)))
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

	fmt.Fprintf(stdout, "Successfully purged %d jobs matching '%s'.\n", int(cleared), match)
}

func updateDependencies(host, jobID string, deps []string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/dependencies", host, url.PathEscape(jobID))

	reqBody := struct {
		DependsOn []string `json:"depends_on"`
	}{
		DependsOn: deps,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal dependency data: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update dependencies: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s dependencies updated to: %s\n", jobID, strings.Join(deps, ", "))
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

func updateAgent(host, jobID, providerStr, modelStr string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/agent", host, jobID)

	reqBody := struct {
		AgentProvider string `json:"agent_provider"`
		AgentModel    string `json:"agent_model"`
	}{
		AgentProvider: providerStr,
		AgentModel:    modelStr,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal agent data: %v\n", err)
		exitFunc(1)
		return
	}

	req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update agent: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s agent updated to provider=%s model=%s\n", jobID, providerStr, modelStr)
}

func updateTimeout(host, jobID, timeoutStr string) {
	urlStr := fmt.Sprintf("%s/jobs/%s/timeout", host, jobID)
	reqBody := fmt.Sprintf(`{"timeout": "%s"}`, timeoutStr)

	req, err := http.NewRequest(http.MethodPut, urlStr, strings.NewReader(reqBody))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to update timeout: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s timeout updated to %s\n", jobID, timeoutStr)
}

func setJobOutput(host, jobID, key, val string) {
	reqBody := struct {
		Outputs map[string]string `json:"outputs"`
	}{
		Outputs: map[string]string{key: val},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal output data: %v\n", err)
		exitFunc(1)
		return
	}

	urlStr := fmt.Sprintf("%s/jobs/%s/output", host, url.PathEscape(jobID))
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to set job output: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully set output %s=%s for job %s\n", key, val, jobID)
}

func setJobProgress(host, jobID string, progress *int, msg *string) {
	reqBody := struct {
		Progress      *int    `json:"progress,omitempty"`
		StatusMessage *string `json:"status_message,omitempty"`
	}{
		Progress:      progress,
		StatusMessage: msg,
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal progress data: %v\n", err)
		exitFunc(1)
		return
	}

	urlStr := fmt.Sprintf("%s/jobs/%s/progress", host, url.PathEscape(jobID))
	req, err := http.NewRequest(http.MethodPut, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to set job progress: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully updated progress for job %s\n", jobID)
}

func addJobMetrics(host, jobID, key string, val float64) {
	reqBody := struct {
		Metrics map[string]float64 `json:"metrics"`
	}{
		Metrics: map[string]float64{key: val},
	}
	payload, err := json.Marshal(reqBody)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal metric data: %v\n", err)
		exitFunc(1)
		return
	}

	urlStr := fmt.Sprintf("%s/jobs/%s/metrics", host, url.PathEscape(jobID))
	req, err := http.NewRequest(http.MethodPost, urlStr, bytes.NewReader(payload))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to add job metrics: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully added metric %s=%.2f for job %s\n", key, val, jobID)
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

func editJob(host, jobID string) {
	// 1. Fetch current job
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
		fmt.Fprintf(stdout, "Failed to decode job response: %v\n", err)
		exitFunc(1)
		return
	}

	if job.Status != "Pending" && job.Status != "Pending Approval" {
		fmt.Fprintf(stdout, "Cannot edit job %s. It is currently %s.\n", jobID, job.Status)
		exitFunc(1)
		return
	}

	// 2. Format WorkItem as JSON
	jobJSON, err := json.MarshalIndent(job.WorkItem, "", "  ")
	if err != nil {
		fmt.Fprintf(stdout, "Failed to format job JSON: %v\n", err)
		exitFunc(1)
		return
	}

	// 3. Create temp file
	tmpFile, err := os.CreateTemp("", fmt.Sprintf("recac-job-%s-*.json", jobID))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create temp file: %v\n", err)
		exitFunc(1)
		return
	}
	defer os.Remove(tmpFile.Name()) // Clean up after we're done

	if _, err := tmpFile.Write(jobJSON); err != nil {
		fmt.Fprintf(stdout, "Failed to write to temp file: %v\n", err)
		exitFunc(1)
		return
	}
	tmpFile.Close()

	// 4. Open editor
	editor := os.Getenv("EDITOR")
	if editor == "" {
		editor = "vi" // Default fallback
	}

	// Wrap editor command in sh -c to properly support arguments in $EDITOR (e.g. "code --wait")
	shellCmd := fmt.Sprintf("%s %s", editor, tmpFile.Name())
	cmd := exec.Command("sh", "-c", shellCmd)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(stdout, "Editor exited with error: %v\n", err)
		exitFunc(1)
		return
	}

	// 5. Read back modified JSON
	modifiedJSON, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		fmt.Fprintf(stdout, "Failed to read modified JSON: %v\n", err)
		exitFunc(1)
		return
	}

	// 6. Validate
	var updatedItem orchestrator.WorkItem
	if err := json.Unmarshal(modifiedJSON, &updatedItem); err != nil {
		fmt.Fprintf(stdout, "Failed to parse modified JSON: %v\n", err)
		exitFunc(1)
		return
	}

	if updatedItem.ID != jobID {
		fmt.Fprintf(stdout, "Error: You cannot change the Job ID during edit.\n")
		exitFunc(1)
		return
	}

	// 7. Send PUT request
	req, err := http.NewRequest(http.MethodPut, fmt.Sprintf("%s/jobs/%s", host, jobID), bytes.NewBuffer(modifiedJSON))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to create PUT request: %v\n", err)
		exitFunc(1)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(putResp.Body)
		fmt.Fprintf(stdout, "Failed to update job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Job %s updated successfully.\n", jobID)
}

func holdJobs(host, match, tag string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/hold", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), nil)
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
		fmt.Fprintf(stdout, "Failed to hold jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Held int `json:"held"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully held %d jobs.\n", result.Held)
}

func unholdJobs(host, match, tag string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/unhold", host))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to parse URL: %v\n", err)
		exitFunc(1)
		return
	}

	q := u.Query()
	if match != "" {
		q.Set("match", match)
	}
	if tag != "" {
		q.Set("tag", tag)
	}
	u.RawQuery = q.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), nil)
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
		fmt.Fprintf(stdout, "Failed to unhold jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Unheld int `json:"unheld"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully unheld %d jobs.\n", result.Unheld)
}

func importJobs(host, path string) {
	file, err := os.Open(path)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to open file %s: %v\n", path, err)
		exitFunc(1)
		return
	}
	defer file.Close()

	var jobs []orchestrator.JobInfo
	if err := json.NewDecoder(file).Decode(&jobs); err != nil {
		fmt.Fprintf(stdout, "Invalid JSON in file %s: %v\n", path, err)
		exitFunc(1)
		return
	}

	var items []orchestrator.WorkItem
	for _, job := range jobs {
		items = append(items, job.WorkItem)
	}

	if len(items) == 0 {
		fmt.Fprintf(stdout, "No jobs found in file %s\n", path)
		return
	}

	data, err := json.Marshal(items)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to encode items: %v\n", err)
		exitFunc(1)
		return
	}

	resp, err := http.Post(fmt.Sprintf("%s/jobs/batch", host), "application/json", bytes.NewReader(data))
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to import jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	body, _ := io.ReadAll(resp.Body)
	fmt.Fprintf(stdout, "Successfully imported jobs:\n%s\n", strings.TrimSpace(string(body)))
}
