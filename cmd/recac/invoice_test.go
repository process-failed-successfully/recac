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

		// Use a fixed reference time (noon today) to ensure stability regardless of when the test runs.
		// This prevents edge cases near midnight where "now" and "now+1h" might span two different days.
		refTime := time.Now().Truncate(24 * time.Hour).Add(12 * time.Hour)

		ts3 := refTime.Add(-25 * time.Hour).Format(time.RFC3339) // Yesterday noon (new session)
		ts1 := refTime.Format(time.RFC3339)                   // Today noon (start of session)
		ts2 := refTime.Add(1 * time.Hour).Format(time.RFC3339) // Today 1 PM (same session)

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
