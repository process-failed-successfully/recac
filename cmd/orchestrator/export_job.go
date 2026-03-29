package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"recac/internal/orchestrator"
)

func exportSingleJob(host, jobID, outPath string) {
	url := fmt.Sprintf("%s/jobs/%s", host, jobID)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Fprintf(stdout, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exitFunc(1)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Fprintf(stdout, "Failed to fetch job %s: status %s - %s\n", jobID, resp.Status, strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	var job orchestrator.JobInfo
	if err := json.NewDecoder(resp.Body).Decode(&job); err != nil {
		fmt.Fprintf(stdout, "Failed to decode job response: %v\n", err)
		exitFunc(1)
		return
	}

	// We extract only the WorkItem payload so it can be re-submitted.
	// Optionally strip ID if we want a clean template? Let's keep it exact for now.
	// The user can modify the JSON file to remove the ID or change it.
	payload, err := json.MarshalIndent(job.WorkItem, "", "  ")
	if err != nil {
		fmt.Fprintf(stdout, "Failed to marshal job to JSON: %v\n", err)
		exitFunc(1)
		return
	}

	if outPath == "" || outPath == "-" {
		fmt.Fprintln(stdout, string(payload))
	} else {
		if err := os.WriteFile(outPath, payload, 0644); err != nil {
			fmt.Fprintf(stdout, "Failed to write exported job to file %s: %v\n", outPath, err)
			exitFunc(1)
			return
		}
		fmt.Fprintf(stdout, "Job %s exported to %s successfully.\n", jobID, outPath)
	}
}
