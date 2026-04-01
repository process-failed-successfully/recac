package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"io"
)

func promoteJob(host, jobID string) {
	url := fmt.Sprintf("%s/jobs/%s/promote", host, jobID)

	req, err := http.NewRequest(http.MethodPost, url, nil)
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
		fmt.Fprintf(stdout, "Failed to promote job: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		fmt.Fprintf(stdout, "Failed to decode response: %v\n", err)
		exitFunc(1)
		return
	}

	priority := result["priority"]
	fmt.Fprintf(stdout, "Successfully promoted job %s to priority %v\n", jobID, priority)
}
