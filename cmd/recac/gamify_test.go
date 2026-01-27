package main

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestGamifyCmd(t *testing.T) {
	// Override gitClientFactory
	originalFactory := gitClientFactory
	defer func() { gitClientFactory = originalFactory }()

	gitClientFactory = func() IGitClient {
		return &MockGitClient{
			RepoExistsFunc: func(repoPath string) bool {
				return true
			},
			LogFunc: func(repoPath string, args ...string) ([]string, error) {
				// Return simulated git log output with tabs
				return []string{
					"COMMIT|abc1234|Alice|2023-10-25 10:00:00 +0000|Initial commit",
					"10\t0\tmain.go",
					"5\t0\tREADME.md",
					"",
					"COMMIT|def5678|Bob|2023-10-26 12:00:00 +0000|Fix bug in main",
					"2\t2\tmain.go",
					"",
					"COMMIT|ghi9012|Alice|2023-10-27 14:00:00 +0000|Add tests",
					"20\t0\tmain_test.go",
				}, nil
			},
		}
	}

	// Create command structure similar to main
	root := &cobra.Command{Use: "recac"}
	// gamifyCmd is a package-level variable in gamify.go
	root.AddCommand(gamifyCmd)

	// Execute
	output, err := executeCommand(root, "gamify")
	assert.NoError(t, err)

	// Verify Output
	assert.Contains(t, output, "GIT LEADERBOARD")
	assert.Contains(t, output, "Alice")
	assert.Contains(t, output, "Bob")

	// Check Alice's rank/medals
	// Alice: 2 commits, ~38 XP
	// Bob: 1 commit, ~30 XP
	// Alice should be rank 1 (🥇)

	// Verify table structure roughly
	lines := strings.Split(strings.TrimSpace(output), "\n")
	foundAlice := false
	foundBob := false
	for _, line := range lines {
		if strings.Contains(line, "Alice") {
			foundAlice = true
			assert.Contains(t, line, "🥇")
			assert.Contains(t, line, "38") // XP
		}
		if strings.Contains(line, "Bob") {
			foundBob = true
			assert.Contains(t, line, "🥈")
			assert.Contains(t, line, "30") // XP
		}
	}
	assert.True(t, foundAlice, "Alice not found in output")
	assert.True(t, foundBob, "Bob not found in output")
}

func TestGamifyCmd_Robustness(t *testing.T) {
	originalFactory := gitClientFactory
	defer func() { gitClientFactory = originalFactory }()

	gitClientFactory = func() IGitClient {
		return &MockGitClient{
			RepoExistsFunc: func(repoPath string) bool { return true },
			LogFunc: func(repoPath string, args ...string) ([]string, error) {
				return []string{
					"COMMIT|abc|User|2023-10-25 10:00:00 +0000|Test",
					"5 0 file with spaces.txt", // Spaces used instead of tabs!
				}, nil
			},
		}
	}

	root := &cobra.Command{Use: "recac"}
	root.AddCommand(gamifyCmd)

	output, err := executeCommand(root, "gamify")
	assert.NoError(t, err)

	// User should get 10 XP (base) + 5 XP (lines/10 is 0) + 5 XP (doc edit .txt) = 15 XP.
	// If parsing fails for filename "file with spaces.txt", it will think filename is "file".
	// "file" has no .txt extension -> No DocEdits bonus.
	// So XP would be 10.

	// Check for 15 XP
	assert.Contains(t, output, "15", "Expected 15 XP (including doc bonus for file with spaces)")
}

func TestGamifyCmd_NoRepo(t *testing.T) {
	// Override gitClientFactory
	originalFactory := gitClientFactory
	defer func() { gitClientFactory = originalFactory }()

	gitClientFactory = func() IGitClient {
		return &MockGitClient{
			RepoExistsFunc: func(repoPath string) bool {
				return false
			},
		}
	}

	root := &cobra.Command{Use: "recac"}
	root.AddCommand(gamifyCmd)

	_, err := executeCommand(root, "gamify")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a git repository")
}
