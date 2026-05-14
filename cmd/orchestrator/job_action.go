package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// jobAction makes a generic POST request to a single-job action endpoint (e.g. demote, promote)
func jobAction(host, jobID, action, successMsg string) {
	u := fmt.Sprintf("%s/jobs/%s/%s", host, jobID, action)

	req, err := http.NewRequest(http.MethodPost, u, nil)
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
		fmt.Fprintf(stdout, "Failed to %s job: %s\n", action, strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	if priority, ok := result["priority"]; ok {
		fmt.Fprintf(stdout, "%s job %s to priority %v\n", successMsg, jobID, priority)
	} else {
		fmt.Fprintf(stdout, "%s job %s\n", successMsg, jobID)
	}
}

// jobBulkAction makes a generic POST request to a bulk-job action endpoint
func jobBulkAction(host, action, match, tag, group, successMsg string) {
	u, err := url.Parse(fmt.Sprintf("%s/jobs/%s/bulk", host, action))
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
		fmt.Fprintf(stdout, "Failed to %s jobs: %s\n", action, strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	// Calculate the past tense of the action
	pastTenseAction := action
	if strings.HasSuffix(action, "e") {
		pastTenseAction += "d"
	} else {
		pastTenseAction += "ed"
	}

	if countInterface, ok := result[pastTenseAction]; ok {
		// handle float64 decoding from generic interface
		if count, isFloat := countInterface.(float64); isFloat {
			fmt.Fprintf(stdout, "%s %d jobs.\n", successMsg, int(count))
		} else {
			fmt.Fprintf(stdout, "%s %v jobs.\n", successMsg, countInterface)
		}
	} else {
		fmt.Fprintf(stdout, "Successfully performed bulk %s.\n", action)
	}
}
