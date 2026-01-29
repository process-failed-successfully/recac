package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// We use the MockGitClient defined in test_helpers_test.go

func TestInvoiceCmd(t *testing.T) {
	// Save original factory
	origFactory := gitClientFactory
	defer func() { gitClientFactory = origFactory }()

	// Setup mock
	mockGit := &MockGitClient{}
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	// Stub config (user.name)
	mockGit.RunFunc = func(dir string, args ...string) (string, error) {
		if len(args) > 1 && args[0] == "config" && args[1] == "user.name" {
			return "Test User", nil
		}
		return "", nil
	}

	// Stub log
	mockGit.LogFunc = func(dir string, args ...string) ([]string, error) {
		// getGitCommits calls client.Log(dir, "--since=30d", "--format=%h|%an|%aI|%s", "--author=Test User")

		now := time.Now()
		ts1 := now.Add(-2 * time.Hour).Format(time.RFC3339)
		ts2 := now.Add(-1 * time.Hour).Format(time.RFC3339) // 1 hour later (same session)
		ts3 := now.Add(-48 * time.Hour).Format(time.RFC3339) // Yesterday (new session)

		return []string{
			fmt.Sprintf("hash1|Test User|%s|Commit 1", ts1),
			fmt.Sprintf("hash2|Test User|%s|Commit 2", ts2),
			fmt.Sprintf("hash3|Test User|%s|Commit 3", ts3),
		}, nil
	}

	// Execute
	// We use `executeCommand` which handles flag resetting and output capturing via rootCmd.

	output, err := executeCommand(rootCmd, "invoice",
		"--client", "MegaCorp",
		"--address", "123 Wall St",
		"--rate", "200",
		"--tax", "10",
		"--since", "30d",
		"--due", "30d")

	assert.NoError(t, err)

	// Assertions
	expectedDue := time.Now().AddDate(0, 0, 30).Format("Jan 02, 2006")
	assert.Contains(t, output, expectedDue) // Check due date
	assert.Contains(t, output, "INVOICE")
	assert.Contains(t, output, "MegaCorp")
	assert.Contains(t, output, "123 Wall St")
	assert.Contains(t, output, "Test User")

	// Calculations:
	// Session 1 (Yesterday): 1 commit. Duration = padding (30m = 0.5h)
	// Session 2 (Today): 2 commits, gap 1h. Duration = 1h + padding (30m) = 1.5h
	// Total Hours = 2.0h
	// Subtotal = 2.0 * 200 = 400.00
	// Tax = 10% of 400 = 40.00
	// Total = 440.00

	assert.Contains(t, output, "0.50") // Session 1 Hours
	assert.Contains(t, output, "1.50") // Session 2 Hours
	assert.Contains(t, output, "400.00") // Subtotal
	assert.Contains(t, output, "40.00") // Tax
	assert.Contains(t, output, "440.00") // Total
}
