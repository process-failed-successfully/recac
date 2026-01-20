package runner

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// EnsureStateIgnored safeguard to ensure agent state files are not tracked by git.
// It checks .gitignore and force-untracks files if they are accidentally tracked.
func EnsureStateIgnored(repoPath string) error {
	stateFiles := []string{
		"agent_state.json",
		".agent_state.json",
		"test_state.json",
		".recac.db",
		"*.pyc",
		"__pycache__/",
		"venv/",
		".venv/",
		"node_modules/",
		"dist/",
		"build/",
		".next/",
		".env",
		".DS_Store",
		"agents/",
	}

	gitignorePath := repoPath + "/.gitignore"
	if err := ensureInGitignore(gitignorePath, stateFiles); err != nil {
		return fmt.Errorf("failed to verify .gitignore: %w", err)
	}

	if err := untrackFiles(repoPath, stateFiles); err != nil {
		return fmt.Errorf("failed to untrack state files: %w", err)
	}

	return nil
}

func ensureInGitignore(gitignorePath string, files []string) error {
	f, err := os.OpenFile(gitignorePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	existing := make(map[string]bool)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		existing[line] = true
	}

	var toAdd []string
	for _, file := range files {
		if !existing[file] {
			toAdd = append(toAdd, file)
		}
	}

	if len(toAdd) > 0 {
		// Append to file
		offset, err := f.Seek(0, 2)
		if err != nil {
			return err
		}

		prefix := "\n"
		if offset == 0 {
			prefix = ""
		}

		if _, err := f.WriteString(prefix + "# Added by RECAC Safeguard\n"); err != nil {
			return err
		}
		for _, file := range toAdd {
			if _, err := f.WriteString(file + "\n"); err != nil {
				return err
			}
		}
	}
	return nil
}

func untrackFiles(repoPath string, files []string) error {
	// Check if files are tracked
	cmd := exec.Command("git", append([]string{"ls-files"}, files...)...)
	cmd.Dir = repoPath
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	if len(output) > 0 {
		trackedFiles := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(trackedFiles) > 0 {
			// Untrack them
			rmCmd := exec.Command("git", append([]string{"rm", "--cached", "-f"}, trackedFiles...)...)
			rmCmd.Dir = repoPath
			if err := rmCmd.Run(); err != nil {
				return fmt.Errorf("failed to untrack files %v: %w", trackedFiles, err)
			}
		}
	}
	return nil
}
