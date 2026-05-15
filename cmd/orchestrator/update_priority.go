package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

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

func updateBulkPriority(host, match, tag, group string, priority int) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/priority", host))
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
	if group != "" {
		q.Set("group", group)
	}
	u.RawQuery = q.Encode()

	reqBody := fmt.Sprintf(`{"priority": %d}`, priority)

	req, err := http.NewRequest(http.MethodPut, u.String(), strings.NewReader(reqBody))
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
		fmt.Fprintf(stdout, "Failed to update bulk priority: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result struct {
		Updated int `json:"updated"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Successfully updated priority for %d jobs.\n", result.Updated)
}
