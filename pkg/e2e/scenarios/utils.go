package scenarios

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

func checkAgentBranchExists(repoPath string) error {
	cmd := exec.Command("git", "branch", "-r")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to list branches: %w", err)
	}

	if !strings.Contains(string(out), "agent/") {
		return fmt.Errorf("no agent branches found")
	}
	return nil
}

func getAgentBranch(repoPath string) (string, error) {
	cmd := exec.Command("git", "branch", "-r")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list branches: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "origin/agent/") {
			return strings.TrimPrefix(line, "origin/"), nil
		}
	}
	return "", fmt.Errorf("no agent branch found")
}

func getSpecificAgentBranch(repoPath, ticketKey string) (string, error) {
	var lastErr error

	// Retry loop to handle eventual consistency or race conditions
	for i := 0; i < 5; i++ {
		// Fetch remote branches first to ensure we see the new agent branch
		fetchCmd := exec.Command("git", "fetch", "origin")
		fetchCmd.Dir = repoPath
		if out, err := fetchCmd.CombinedOutput(); err != nil {
			lastErr = fmt.Errorf("failed to fetch branches: %w (output: %s)", err, string(out))
			time.Sleep(2 * time.Second)
			continue
		}

		cmd := exec.Command("git", "branch", "-r")
		cmd.Dir = repoPath
		out, err := cmd.Output()
		if err != nil {
			lastErr = fmt.Errorf("failed to list branches: %w", err)
			time.Sleep(1 * time.Second)
			continue
		}

		outputStr := string(out)
		// Branch pattern usually agent/TICKET-ID-TIMESTAMP or similar
		// We check for TICKET-ID
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			// Look for agent/.*KEY.*
			if strings.Contains(line, "origin/agent/") && strings.Contains(line, ticketKey) {
				return strings.TrimPrefix(line, "origin/"), nil
			}
		}

		lastErr = fmt.Errorf("no agent branch found for ticket %s (available: %s)", ticketKey, outputStr)
		time.Sleep(2 * time.Second)
	}

	return "", lastErr
}
