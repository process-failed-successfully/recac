package main

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

func pauseOrchestratorGroup(host, group string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/groups/%s/pause", host, group), nil)
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
		fmt.Fprintf(stdout, "Failed to pause concurrency group: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Concurrency group %s paused.\n", group)
}

func resumeOrchestratorGroup(host, group string) {
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("%s/groups/%s/resume", host, group), nil)
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
		fmt.Fprintf(stdout, "Failed to resume concurrency group: %s\n", strings.TrimSpace(string(body)))
		exitFunc(1)
		return
	}

	fmt.Fprintf(stdout, "Concurrency group %s resumed.\n", group)
}
