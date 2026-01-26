package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
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

func TestGetGitCommits_Arguments(t *testing.T) {
	originalFactory := gitClientFactory
	defer func() { gitClientFactory = originalFactory }()

	mockClient := &MockGitClient{
		LogFunc: func(repoPath string, args ...string) ([]string, error) {
			// Check if "log" is passed as an argument.
			// It should NOT be, because Log() method prepends it in implementation.
			if len(args) > 0 && args[0] == "log" {
				return nil, fmt.Errorf("unexpected argument 'log'")
			}
			return []string{}, nil
		},
	}

	gitClientFactory = func() IGitClient {
		return mockClient
	}

	_, err := getGitCommits(".", "24h", "")
	assert.NoError(t, err)
}
