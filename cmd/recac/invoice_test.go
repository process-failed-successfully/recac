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
		// Ensure ts1 and ts2 are definitely "today" and ts3 is "yesterday"
		// even if running near midnight.
		// Session 2 (Today): Gap 1h.
		// We use small offsets from now to keep it in the same day (unless now is exactly 00:00:00)
		// To be safe against midnight crossing, we could mock now, but simply using recent times is usually enough.
		// If now is 00:30, -2h is yesterday.
		// Let's use -60m and -0m (now) to be safe? No, we need 1h gap.
		// Let's use hardcoded dates? No, logic depends on "Since 30d".
		// Best approach: Use 25h ago for yesterday. For today, use 10m ago and 70m ago (1h gap).
		// But 70m ago might cross midnight if now is 01:00.
		// Let's just force the timestamps to be clearly separated days if possible, or accept that
		// if they fall on same day, the test fails.
		// To fix reliably: set "now" to noon today for calculation purposes? We can't easily mock time.Now inside the command.
		// So we construct timestamps relative to a fixed "noon today".
		noonToday := time.Date(now.Year(), now.Month(), now.Day(), 12, 0, 0, 0, now.Location())
		ts1 := noonToday.Add(-1 * time.Hour).Format(time.RFC3339) // 11:00 Today
		ts2 := noonToday.Format(time.RFC3339)                     // 12:00 Today (1h gap)
		ts3 := noonToday.Add(-25 * time.Hour).Format(time.RFC3339) // 11:00 Yesterday (new session)

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
