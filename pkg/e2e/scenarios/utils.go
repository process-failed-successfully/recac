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
		// Use git ls-remote to check the server directly, bypassing local fetch/refspec issues
		// We look for refs/heads/agent/*TICKET*
		// Note: The pattern needs to match the remote ref name.
		// Agent branches are typically "agent/TICKET" or "agent/TICKET-SUFFIX"
		pattern := fmt.Sprintf("refs/heads/agent/*%s*", ticketKey)
		cmd := exec.Command("git", "ls-remote", "--heads", "origin", pattern)
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
				ref := parts[1] // refs/heads/agent/MFLP-4968
				if strings.Contains(ref, ticketKey) {
					// We found it on remote. Now verify we can use it.
					// For the verification logic to work, we might need to fetch it to local if checking out.
					// But the caller just needs the branch name (e.g. agent/MFLP-4968) to checkout or diff.
					// We return "agent/MFLP-4968" (without origin/ or refs/heads/)
					branchName := strings.TrimPrefix(ref, "refs/heads/")

					// Explicitly fetch this branch to ensure it's available locally
					fetchCmd := exec.Command("git", "fetch", "origin", branchName+":"+branchName)
					fetchCmd.Dir = repoPath
					if err := fetchCmd.Run(); err != nil {
						// If direct fetch fails (e.g. permission), fallback to simple fetch
						// but mostly we just return the name found.
						// We'll log warning but proceed as ls-remote confirmed existence.
						fmt.Printf("Warning: failed to fetch specific branch %s: %v\n", branchName, err)
					}

					return branchName, nil
				}
			}
		}

		lastErr = fmt.Errorf("parsed ls-remote output but found no match for %s in: %s", ticketKey, outputStr)
		time.Sleep(2 * time.Second)
	}

	return "", lastErr
}
