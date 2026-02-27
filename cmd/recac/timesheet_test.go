package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseGitLogOutput(t *testing.T) {
	// 2023-10-27T10:00:00Z
	log := []string{
		"a1b2c3d|jules|2023-10-27T10:00:00Z|Initial commit",
		"e5f6g7h|jules|2023-10-27T10:15:00Z|Second commit",
		"i9j0k1l|jules|2023-10-27T10:30:00Z|Third commit",
	}
	commits, err := parseGitLogOutput(log)
	assert.NoError(t, err)
	assert.Len(t, commits, 3)

	assert.Equal(t, "a1b2c3d", commits[0].Hash)
	assert.Equal(t, "jules", commits[0].Author)
	assert.Equal(t, "Initial commit", commits[0].Message)

	// Verify sorting (input is ascending already, but function sorts ascending)
	assert.True(t, commits[0].Timestamp.Before(commits[1].Timestamp))
}

func TestCalculateSessions_SingleSession(t *testing.T) {
	baseTime := time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC)
	commits := []Commit{
		{Timestamp: baseTime},
		{Timestamp: baseTime.Add(15 * time.Minute)}, // +15m
		{Timestamp: baseTime.Add(30 * time.Minute)}, // +30m (total gap 15m)
	}

	threshold := 60 * time.Minute
	padding := 30 * time.Minute

	sessions := calculateSessions(commits, threshold, padding)

	assert.Len(t, sessions, 1)
	assert.Equal(t, 3, sessions[0].Commits)
	// Duration = (10:30 - 10:00) + 30m = 30m + 30m = 60m = 1h
	assert.Equal(t, 1.0, sessions[0].Duration)
}

func TestCalculateSessions_MultipleSessions(t *testing.T) {
	baseTime := time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC)
	commits := []Commit{
		// Session 1
		{Timestamp: baseTime},
		{Timestamp: baseTime.Add(15 * time.Minute)},

		// Session 2 (Gap 2 hours)
		{Timestamp: baseTime.Add(135 * time.Minute)}, // 10:00 + 135m = 12:15. Gap from 10:15 is 120m (2h).
		// Wait, 12:15 - 10:15 = 2h. If threshold is 60m, this is a break.
	}

	threshold := 60 * time.Minute
	padding := 30 * time.Minute

	sessions := calculateSessions(commits, threshold, padding)

	assert.Len(t, sessions, 2)

	// Session 1: 10:00 - 10:15. Duration 15+30 = 45m = 0.75h
	assert.Equal(t, 0.75, sessions[0].Duration)
	assert.Equal(t, 2, sessions[0].Commits)

	// Session 2: 12:15 - 12:15. Single commit. Duration 0+30 = 30m = 0.5h
	assert.Equal(t, 0.5, sessions[1].Duration)
	assert.Equal(t, 1, sessions[1].Commits)
}

func TestAggregateTimesheet(t *testing.T) {
	sessions := []Session{
		{Duration: 1.0, StartTime: time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC)},
		{Duration: 0.5, StartTime: time.Date(2023, 10, 27, 12, 0, 0, 0, time.UTC)},
		{Duration: 2.0, StartTime: time.Date(2023, 10, 28, 0, 0, 0, 0, time.UTC)}, // Next day
	}

	rate := 100.0
	report := aggregateTimesheet(sessions, rate)

	assert.Equal(t, 3.5, report.TotalHours)
	assert.Equal(t, 3, report.TotalSessions)
	assert.Equal(t, 350.0, report.TotalCost)

	assert.Equal(t, 1.5, report.DailyStats["2023-10-27"])
	assert.Equal(t, 2.0, report.DailyStats["2023-10-28"])
}

func TestPrintTimesheetTable(t *testing.T) {
	report := TimesheetReport{
		TotalHours:    10.5,
		TotalSessions: 5,
		TotalCost:     1050.0,
		DailyStats: map[string]float64{
			"2023-10-27": 5.5,
			"2023-10-28": 5.0,
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	printTimesheetTable(report, "jules", "24h", 100.0)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "Timesheet Report")
	assert.Contains(t, output, "Author: jules")
	assert.Contains(t, output, "Period: Since 24h")
	assert.Contains(t, output, "2023-10-27")
	assert.Contains(t, output, "5.50 hrs")
	// tabwriter replaces tabs with spaces, so we check for substring parts or use regex if needed
	// Or just check that the values are present in the line
	assert.Contains(t, output, "Total Hours:")
	assert.Contains(t, output, "10.50 hrs")
	assert.Contains(t, output, "Estimated Cost:")
	assert.Contains(t, output, "$1050.00")
}

// Helper to mock GitClient
type MockTimesheetGitClient struct {
	LogOutput []string
	RunOutput string
}

func (m *MockTimesheetGitClient) Checkout(repoPath, commitOrBranch string) error { return nil }
func (m *MockTimesheetGitClient) Diff(repoPath, commitA, commitB string) (string, error) { return "", nil }
func (m *MockTimesheetGitClient) DiffStaged(repoPath string) (string, error) { return "", nil }
func (m *MockTimesheetGitClient) DiffStat(repoPath, commitA, commitB string) (string, error) { return "", nil }
func (m *MockTimesheetGitClient) CurrentCommitSHA(repoPath string) (string, error) { return "", nil }
func (m *MockTimesheetGitClient) RepoExists(repoPath string) bool { return true }
func (m *MockTimesheetGitClient) Commit(repoPath, message string) error { return nil }
func (m *MockTimesheetGitClient) Log(repoPath string, args ...string) ([]string, error) {
	return m.LogOutput, nil
}
func (m *MockTimesheetGitClient) Fetch(repoPath, remote, branch string) error { return nil }
func (m *MockTimesheetGitClient) CurrentBranch(repoPath string) (string, error) { return "", nil }
func (m *MockTimesheetGitClient) CheckoutNewBranch(repoPath, branch string) error { return nil }
func (m *MockTimesheetGitClient) BisectStart(repoPath, bad, good string) error { return nil }
func (m *MockTimesheetGitClient) BisectGood(repoPath, rev string) error { return nil }
func (m *MockTimesheetGitClient) BisectBad(repoPath, rev string) error { return nil }
func (m *MockTimesheetGitClient) BisectReset(repoPath string) error { return nil }
func (m *MockTimesheetGitClient) BisectLog(repoPath string) ([]string, error) { return nil, nil }
func (m *MockTimesheetGitClient) Tag(repoPath, version string) error { return nil }
func (m *MockTimesheetGitClient) DeleteTag(repoPath, version string) error { return nil }
func (m *MockTimesheetGitClient) PushTags(repoPath string) error { return nil }
func (m *MockTimesheetGitClient) LatestTag(repoPath string) (string, error) { return "", nil }
func (m *MockTimesheetGitClient) Run(repoPath string, args ...string) (string, error) {
	return m.RunOutput, nil
}
func (m *MockTimesheetGitClient) DeleteLocalBranch(repoPath, branch string) error { return nil }
func (m *MockTimesheetGitClient) CreatePR(repoPath, title, body, base string) (string, error) { return "", nil }

func (m *MockTimesheetGitClient) StashPush(d, msg string) error { return nil }
func (m *MockTimesheetGitClient) StashList(d string) ([]string, error) { return nil, nil }
func (m *MockTimesheetGitClient) StashShow(d, id string) (string, error) { return "", nil }
func (m *MockTimesheetGitClient) StashApply(d, id string) error { return nil }
func (m *MockTimesheetGitClient) StashDrop(d, id string) error { return nil }
func (m *MockTimesheetGitClient) StashClear(d string) error { return nil }
func (m *MockTimesheetGitClient) AbortMerge(d string) error { return nil }
func (m *MockTimesheetGitClient) Recover(d string) error { return nil }
func (m *MockTimesheetGitClient) Clean(d string) error { return nil }
func (m *MockTimesheetGitClient) ResetHard(d, remote, branch string) error { return nil }
func (m *MockTimesheetGitClient) StashPop(d string) error { return nil }
func (m *MockTimesheetGitClient) DeleteRemoteBranch(d, remote, branch string) error { return nil }
func (m *MockTimesheetGitClient) SetRemoteURL(d, name, url string) error { return nil }
func (m *MockTimesheetGitClient) LocalBranchExists(d, branch string) (bool, error) { return false, nil }
func (m *MockTimesheetGitClient) Config(d, key, value string) error { return nil }
func (m *MockTimesheetGitClient) ConfigGlobal(key, value string) error { return nil }
func (m *MockTimesheetGitClient) ConfigAddGlobal(key, value string) error { return nil }
func (m *MockTimesheetGitClient) RemoteBranchExists(d, remote, branch string) (bool, error) { return false, nil }
func (m *MockTimesheetGitClient) Clone(ctx context.Context, repoURL, d string) error { return nil }
func (m *MockTimesheetGitClient) Push(d, branch string) error { return nil }
func (m *MockTimesheetGitClient) Pull(d, remote, branch string) error { return nil }
func (m *MockTimesheetGitClient) Stash(d string) error { return nil }
func (m *MockTimesheetGitClient) Merge(d, branchName string) error { return nil }

func TestRunTimesheet(t *testing.T) {
	origFactory := gitClientFactory
	defer func() { gitClientFactory = origFactory }()

	mockGit := &MockTimesheetGitClient{
		RunOutput: "jules",
		LogOutput: []string{
			"a1b2c3d|jules|2023-10-27T10:00:00Z|Initial commit",
			"e5f6g7h|jules|2023-10-27T10:15:00Z|Second commit",
		},
	}
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	// Use a dummy root command to execute timesheetCmd
	// We need to be careful as timesheetCmd is global and might be attached to the real rootCmd
	// But adding it to another command for testing is usually fine
	root := &cobra.Command{Use: "test-root"}
	root.AddCommand(timesheetCmd)

	// Need to set output on root as well
	root.SetOut(w)
	root.SetErr(w)

	root.SetArgs([]string{"timesheet", "--since=24h", "--rate=100"})
	err := root.Execute()
	require.NoError(t, err)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	assert.Contains(t, output, "Timesheet Report")
	assert.Contains(t, output, "Author: jules")
	assert.Contains(t, output, "Total Hours:")
}
