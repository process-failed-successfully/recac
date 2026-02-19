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

// HTTPClient interface for mocking
type HTTPClient interface {
	Post(url, contentType string, body io.Reader) (*http.Response, error)
}

func submitJob(host, filePath string, client HTTPClient, w io.Writer, exit func(int)) {
	file, err := os.Open(filePath)
	if err != nil {
		fmt.Fprintf(w, "Failed to open file %s: %v\n", filePath, err)
		exit(1)
		return
	}
	defer file.Close()

	// Verify JSON validity before sending (optional but good UX)
	var item map[string]interface{}
	if err := json.NewDecoder(file).Decode(&item); err != nil {
		fmt.Fprintf(w, "Invalid JSON in file %s: %v\n", filePath, err)
		exit(1)
		return
	}
	// Reset file pointer
	if _, err := file.Seek(0, 0); err != nil {
		fmt.Fprintf(w, "Failed to seek file %s: %v\n", filePath, err)
		exit(1)
		return
	}

	resp, err := client.Post(fmt.Sprintf("%s/jobs", host), "application/json", file)
	if err != nil {
		fmt.Fprintf(w, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exit(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(w, "Failed to submit job: %s\n", strings.TrimSpace(string(body)))
		exit(1)
		return
	}

	fmt.Fprintf(w, "%s\n", strings.TrimSpace(string(body)))
}

func submitAdHocJob(host, repo, task, id string, client HTTPClient, w io.Writer, exit func(int)) {
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
		fmt.Fprintf(w, "Failed to marshal work item: %v\n", err)
		exit(1)
		return
	}

	resp, err := client.Post(fmt.Sprintf("%s/jobs", host), "application/json", bytes.NewBuffer(payload))
	if err != nil {
		fmt.Fprintf(w, "Failed to connect to orchestrator at %s: %v\n", host, err)
		exit(1)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusAccepted {
		fmt.Fprintf(w, "Failed to submit job: %s\n", strings.TrimSpace(string(body)))
		exit(1)
		return
	}

	fmt.Fprintf(w, "%s\n", strings.TrimSpace(string(body)))
}
