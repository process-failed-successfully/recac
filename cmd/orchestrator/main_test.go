package main

import (
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestMetricsServer(t *testing.T) {
	// Use a random high port to avoid conflicts
	port := 34569

	// Create a dummy work file for the file poller
	tmpDir := t.TempDir()
	workFile := filepath.Join(tmpDir, "work.json")
	if err := os.WriteFile(workFile, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}

	// Run the orchestrator using "go run ."
	// Note: We use "." so it builds the package. ensure we are in the correct dir or use absolute path?
	// The test runs in the directory of the test file.

	cmd := exec.Command("go", "run", ".",
		"--poller", "file",
		"--work-file", workFile,
		"--mode", "local",
		"--metrics-port", fmt.Sprintf("%d", port),
		"--verbose",
	)

	// Pipe output to stdout/stderr for debugging if test fails
	// cmd.Stdout = os.Stdout
	// cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start orchestrator: %v", err)
	}

	// Ensure we kill the process at the end
	defer func() {
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}()

	// Poll the metrics endpoint
	url := fmt.Sprintf("http://localhost:%d/metrics", port)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.After(10 * time.Second)

	for {
		select {
		case <-timeout:
			t.Fatalf("Timeout waiting for metrics server at %s", url)
		case <-ticker.C:
			// check if process is still running
			if cmd.ProcessState != nil && cmd.ProcessState.Exited() {
				t.Fatalf("Orchestrator process exited unexpectedly")
			}

			resp, err := http.Get(url)
			if err == nil {
				resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					t.Logf("Successfully connected to metrics server at %s", url)
					return
				}
			}
		}
	}
}
