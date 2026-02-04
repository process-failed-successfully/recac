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

	// Retry loop to handle eventual consistency
	for i := 0; i < 5; i++ {
		// Use git ls-remote to check the server directly
		// Broader pattern to ensure we match "refs/heads/agent/..." or similar
		pattern := fmt.Sprintf("*%s*", ticketKey)
		cmd := exec.Command("git", "ls-remote", "origin", pattern)
		cmd.Dir = repoPath

		out, err := cmd.Output()
		if err != nil {
			lastErr = fmt.Errorf("failed to list remote branches: %w", err)
			time.Sleep(2 * time.Second)
			continue
		}

		outputStr := strings.TrimSpace(string(out))
		if outputStr == "" {
			lastErr = fmt.Errorf("no remote branch found for ticket %s (pattern: %s)", ticketKey, pattern)
			time.Sleep(2 * time.Second)
			continue
		}

		// ls-remote output format: <hash>\t<ref>
		// e.g. "hash    refs/heads/agent/MFLP-4968"
		lines := strings.Split(outputStr, "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				ref := parts[1]
				// Check for agent branch specifically to avoid matching other tags/branches
				if strings.Contains(ref, "/agent/") && strings.Contains(ref, ticketKey) {
					// Extract branch name for fetch
					branchName := strings.TrimPrefix(ref, "refs/heads/")

					// Explicitly fetch this branch to ensure it's available locally
					// Using refspec +refs/heads/BRANCH:refs/remotes/origin/BRANCH to map it correctly
					// or just fetch it to FETCH_HEAD.
					// The caller likely expects a branch name they can checkout or diff.
					// If we fetch to refs/remotes/origin/..., then "agent/MFLP..." works if it resolves to that.
					// Let's fetch it to a local tracking branch or just ensure origin/BRANCH exists.
					fetchCmd := exec.Command("git", "fetch", "origin", fmt.Sprintf("%s:%s", ref, ref))
					fetchCmd.Dir = repoPath
					if err := fetchCmd.Run(); err != nil {
						// Fallback: try simple fetch of the branch name
						_ = exec.Command("git", "fetch", "origin", branchName).Run()
					}

					return branchName, nil
				}
			}
		}

		lastErr = fmt.Errorf("parsed ls-remote output but found no agent branch for %s in: %s", ticketKey, outputStr)
		time.Sleep(2 * time.Second)
	}

	return "", lastErr
}
