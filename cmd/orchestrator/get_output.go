package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"recac/internal/orchestrator"
)

func getJobOutput(host, jobID, key string) {
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

	if key != "" {
		val, exists := job.Outputs[key]
		if !exists {
			fmt.Fprintf(stdout, "Error: Output key '%s' not found for job %s\n", key, jobID)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "%s\n", val)
	} else {
		if job.Outputs == nil || len(job.Outputs) == 0 {
			fmt.Fprintf(stdout, "{}\n")
			return
		}
		encoder := json.NewEncoder(stdout)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(job.Outputs); err != nil {
			fmt.Fprintf(stdout, "Failed to encode outputs to JSON: %v\n", err)
			exitFunc(1)
		}
	}
}
