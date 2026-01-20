package scenarios

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

type DistributedLogScenario struct{}

func (s *DistributedLogScenario) Name() string {
	return "distributed-log"
}

func (s *DistributedLogScenario) Description() string {
	return "Build a persistent append-only log server (Mini-Kafka)."
}

func (s *DistributedLogScenario) AppSpec(repoURL string) string {
	return fmt.Sprintf(`### ID:[LOG] Distributed Log Server

Build an HTTP server that acts as a simple distributed log.

The server MUST implement two endpoints:
1. POST /produce: Accepts a JSON body {"data": "..."}. It appends the data to a persistent log file and returns the "offset" (the index/position of the entry).
2. GET /consume?offset=<n>: Returns the data at the specified offset. If the offset doesn't exist, return 404.

CRITICAL: The data MUST be persistent. If the server restarts, previously produced data MUST still be available. Do not use in-memory arrays for the primary storage.

Use any language (Python or Go preferred).
Ensure you use a bash block to create the source files.

Repo: %s`, repoURL)
}

func (s *DistributedLogScenario) Generate(uniqueID string, repoURL string) []TicketSpec {
	return []TicketSpec{
		{
			ID:      "LOG",
			Summary: fmt.Sprintf("[%s] Build a Distributed Log Server", uniqueID),
			Desc: fmt.Sprintf(`Build an HTTP server that acts as a simple distributed log.

The server MUST implement two endpoints:
1. POST /produce: Accepts a JSON body {"data": "..."}. It appends the data to a persistent log file and returns the "offset" (the index/position of the entry).
2. GET /consume?offset=<n>: Returns the data at the specified offset. If the offset doesn't exist, return 404.

CRITICAL: The data MUST be persistent. If the server restarts, previously produced data MUST still be available. Do not use in-memory arrays for the primary storage.

Use any language (Python or Go preferred).
Ensure you use a bash block to create the source files.
Commit the code and ensure it is ready for execution.

Repo: %s`, repoURL),
			Type: "Task",
		},
	}
}

func (s *DistributedLogScenario) Verify(repoPath string, ticketKeys map[string]string) error {
	ticketKey, ok := ticketKeys["LOG"]
	if !ok {
		return fmt.Errorf("LOG ticket key not found")
	}

	branch, err := getSpecificAgentBranch(repoPath, ticketKey)
	if err != nil {
		return fmt.Errorf("branch for %s not found: %w", ticketKey, err)
	}

	// Checkout branch
	checkoutCmd := exec.Command("git", "checkout", branch)
	checkoutCmd.Dir = repoPath
	if out, err := checkoutCmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to checkout %s: %v\nOutput: %s", branch, err, out)
	}

	serverURL := "http://localhost:8080"

	startServer := func() (*exec.Cmd, error) {
		var runCmd *exec.Cmd
		if _, err := exec.LookPath("go"); err == nil {
			runCmd = exec.Command("go", "run", ".")
			runCmd.Dir = repoPath
		} else if _, err := exec.LookPath("python3"); err == nil {
			runCmd = exec.Command("python3", "main.py")
			runCmd.Dir = repoPath
		}
		if runCmd == nil {
			return nil, fmt.Errorf("could not determine how to run the log server")
		}
		if err := runCmd.Start(); err != nil {
			return nil, err
		}
		// Wait for ready
		for i := 0; i < 20; i++ {
			resp, err := http.Get(serverURL + "/consume?offset=0")
			if err == nil {
				resp.Body.Close()
				return runCmd, nil
			}
			time.Sleep(500 * time.Millisecond)
		}
		return nil, fmt.Errorf("server failed to start")
	}

	// 1. Start Server
	cmd1, err := startServer()
	if err != nil {
		return err
	}

	// 2. Produce some data
	produce := func(data string) error {
		resp, err := http.Post(serverURL+"/produce", "application/json", bytes.NewBufferString(fmt.Sprintf(`{"data": "%s"}`, data)))
		if err != nil {
			return err
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
			return fmt.Errorf("produce failed with status %d", resp.StatusCode)
		}
		return nil
	}

	if err := produce("first-entry"); err != nil {
		cmd1.Process.Kill()
		return err
	}
	if err := produce("second-entry"); err != nil {
		cmd1.Process.Kill()
		return err
	}

	// 3. Restart Server (Verify Persistence)
	cmd1.Process.Kill()
	fmt.Println("Server killed, restarting to verify persistence...")
	time.Sleep(1 * time.Second)

	cmd2, err := startServer()
	if err != nil {
		return err
	}
	defer cmd2.Process.Kill()

	// 4. Consume and Verify
	consume := func(offset int) (string, error) {
		resp, err := http.Get(fmt.Sprintf("%s/consume?offset=%d", serverURL, offset))
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("consume failed with status %d", resp.StatusCode)
		}
		body, _ := io.ReadAll(resp.Body)
		return string(body), nil
	}

	data0, err := consume(0)
	if err != nil || !strings.Contains(data0, "first-entry") {
		return fmt.Errorf("persistence check failed for offset 0: got %q (err: %v)", data0, err)
	}

	data1, err := consume(1)
	if err != nil || !strings.Contains(data1, "second-entry") {
		return fmt.Errorf("persistence check failed for offset 1: got %q (err: %v)", data1, err)
	}

	fmt.Println("Persistence check passed!")
	return nil
}

func init() {
	Register(&DistributedLogScenario{})
}
