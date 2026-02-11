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

func listRemoteAgentBranches(repoPath string) ([]string, error) {
	cmd := exec.Command("git", "branch", "-r")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list branches: %w", err)
	}

	var branches []string
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "origin/agent/") {
			branch := strings.TrimPrefix(line, "origin/")
			branches = append(branches, branch)
		}
	}
	return branches, nil
}

func getLatestAgentBranch(repoPath string) (string, error) {
	// Sort refs by committer date descending and take the top one
	cmd := exec.Command("git", "for-each-ref", "--sort=-committerdate", "--format=%(refname:short)", "refs/remotes/origin/agent/")
	cmd.Dir = repoPath
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to list agent branches: %w", err)
	}

	lines := strings.Split(string(out), "\n")
	if len(lines) > 0 && lines[0] != "" {
		return strings.TrimPrefix(strings.TrimSpace(lines[0]), "origin/"), nil
	}
	return "", fmt.Errorf("no agent branches found")
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
