package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

func healJob(host, jobID string, wait bool) {
	fmt.Fprintf(stdout, "Healing job %s...\n", jobID)

	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/jobs/%s/heal", host, jobID), nil)
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

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(stdout, "Failed to submit healed job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		HealedJobID string `json:"healed_job_id"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Healed job %s submitted successfully.\n", result.HealedJobID)

	if wait {
		if err := waitForJob(host, result.HealedJobID, stdout); err != nil {
			fmt.Fprintf(stdout, "Healed job failed: %v\n", err)
			exitFunc(1)
			return
		}
	}
}

func healBulkJobs(host, match, tag string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/heal/bulk", host))
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
		fmt.Fprintf(stdout, "Failed to heal jobs: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Healed int `json:"healed"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully healed %d failed jobs.\n", result.Healed)
}
