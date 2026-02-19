package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"recac/internal/orchestrator"

	"github.com/google/uuid"
)

func submitJob(host, filePath string) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Printf("Failed to open file %s: %v\n", filePath, err)
		os.Exit(1)
	}
	defer file.Close()

	// Verify JSON validity before sending (optional but good UX)
	var item map[string]interface{}
	if err := json.NewDecoder(file).Decode(&item); err != nil {
		fmt.Printf("Invalid JSON in file %s: %v\n", filePath, err)
		os.Exit(1)
	}
	// Reset file pointer
	file.Seek(0, 0)

	resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", file)
	if err != nil {
		fmt.Printf("Failed to connect to orchestrator at %s: %v\n", host, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Printf("Failed to submit job: %s\n", strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	fmt.Printf("%s\n", strings.TrimSpace(string(body)))
}

func submitAdHocJob(host, repo, task, id string) {
	if id == "" {
		id = uuid.New().String()
	}

	item := orchestrator.WorkItem{
		ID:          id,
		Summary:     task, // Using task description as summary for ad-hoc
		Description: task,
		RepoURL:     repo,
		// No EnvVars for now, could add if needed
	}

	payload, err := json.Marshal(item)
	if err != nil {
		fmt.Printf("Failed to marshal work item: %v\n", err)
		os.Exit(1)
	}

	resp, err := http.Post(fmt.Sprintf("%s/jobs", host), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Printf("Failed to connect to orchestrator at %s: %v\n", host, err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Printf("Failed to submit job: %s\n", strings.TrimSpace(string(body)))
		os.Exit(1)
	}

	fmt.Printf("%s\n", strings.TrimSpace(string(body)))
}
