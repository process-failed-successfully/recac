package scenarios

import (
	"fmt"
	"os/exec"
	"strings"
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
	cmd := exec.Command("git", "branch", "-r")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list branches: %w", err)
	}

	// Branch pattern usually agent/TICKET-ID-TIMESTAMP or similar
	// We check for TICKET-ID
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Look for agent/.*KEY.*
		if strings.Contains(line, "origin/agent/") && strings.Contains(line, ticketKey) {
			return strings.TrimPrefix(line, "origin/"), nil
		}
	}
	return "", fmt.Errorf("no agent branch found for ticket %s", ticketKey)
}
