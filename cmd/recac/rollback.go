package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"recac/internal/git"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(rollbackCmd)
}

var rollbackCmd = &cobra.Command{
	Use:   "rollback [ITERATION]",
	Short: "Rollback the workspace to a specific agent iteration",
	Long:  `Reverts the workspace to the state of a specific agent iteration.
This is useful for undoing "bad" agent runs or hallucinations.

The command searches the git log for commits matching "chore: progress update (iteration X)"
and performs a hard reset to that commit.

If no iteration is provided, it lists available rollback points.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		workspace, _ := os.Getwd()
		// If inside a subdirectory, find root
		gitClient := git.NewClient()
		if !gitClient.RepoExists(workspace) {
			// Try to find up
			parent := workspace
			found := false
			for i := 0; i < 5; i++ {
				if gitClient.RepoExists(parent) {
					workspace = parent
					found = true
					break
				}
				parent = filepath.Dir(parent)
			}
			if !found {
				fmt.Fprintf(os.Stderr, "Error: Current directory is not a git repository.\n")
				os.Exit(1)
			}
		}

		if len(args) == 0 {
			listRollbackPoints(workspace)
			return
		}

		iteration, err := strconv.Atoi(args[0])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: Iteration must be an integer.\n")
			os.Exit(1)
		}

		if err := performRollback(workspace, iteration); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	},
}

func listRollbackPoints(workspace string) {
	fmt.Println("Available Rollback Points:")
	// git log --grep="chore: progress update (iteration " --format="%h %s"
	cmd := exec.Command("git", "log", "--grep=chore: progress update (iteration ", "--format=%h %s", "-n", "20")
	cmd.Dir = workspace
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Warning: failed to list commits: %v\n", err)
	}
}

// Local variable for testability
var getCommitForIteration = func(workspace string, iteration int) (string, error) {
    pattern := fmt.Sprintf("chore: progress update (iteration %d)", iteration)
    return findCommitInLog(workspace, pattern)
}

func findCommitInLog(workspace, pattern string) (string, error) {
    cmd := exec.Command("git", "log", "--grep="+pattern, "--fixed-strings", "--format=%H", "-n", "1")
    cmd.Dir = workspace
    out, err := cmd.Output()
    if err != nil {
        return "", fmt.Errorf("failed to search git log: %w", err)
    }

    sha := strings.TrimSpace(string(out))
    if sha == "" {
        return "", fmt.Errorf("commit not found for pattern: %s", pattern)
    }
    return sha, nil
}

func performRollback(workspace string, iteration int) error {
	fmt.Printf("Searching for iteration %d in %s...\n", iteration, workspace)

	sha, err := getCommitForIteration(workspace, iteration)
	if err != nil {
		return err
	}

	fmt.Printf("Found commit %s. Rolling back...\n", sha)

    if err := resetHardToCommit(workspace, sha); err != nil {
        return fmt.Errorf("git reset failed: %w", err)
    }

    // Cleanup .agent_state.json
    statePath := filepath.Join(workspace, ".agent_state.json")
    if err := os.Remove(statePath); err != nil && !os.IsNotExist(err) {
        fmt.Printf("Warning: Failed to remove .agent_state.json: %v\n", err)
    } else {
        fmt.Println("Cleared agent state cache.")
    }

    fmt.Printf("Successfully rolled back to iteration %d.\n", iteration)
    return nil
}

var resetHardToCommit = func(workspace, sha string) error {
    cmd := exec.Command("git", "reset", "--hard", sha)
    cmd.Dir = workspace
    out, err := cmd.CombinedOutput()
    if err != nil {
        return fmt.Errorf("%v: %s", err, string(out))
    }
    return nil
}
