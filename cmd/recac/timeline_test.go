package main

import (
	"bytes"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProcessCommits(t *testing.T) {
	lines := []string{
		"hash1|2023-10-01T10:00:00Z|feat(auth): add login",
		"hash2|2023-10-02T10:00:00Z|fix(auth): fix logout",
		"hash3|2023-10-05T10:00:00Z|feat(ui): new button",
		"hash4|2023-10-06T10:00:00Z|chore: cleanup",
	}

	events := processCommits(lines)

	assert.Len(t, events, 3)

	findEvent := func(scope string) *TimelineEvent {
		for _, e := range events {
			if e.Scope == scope {
				return &e
			}
		}
		return nil
	}

	auth := findEvent("auth")
	require.NotNil(t, auth)
	assert.Equal(t, "feat", auth.Type)
	assert.Equal(t, "2023-10-01", auth.StartDate.Format("2006-01-02"))
	assert.Equal(t, "2023-10-02", auth.EndDate.Format("2006-01-02"))
	assert.Len(t, auth.Commits, 2)

	ui := findEvent("ui")
	require.NotNil(t, ui)
	assert.Equal(t, "feat", ui.Type)
	assert.Equal(t, "2023-10-05", ui.StartDate.Format("2006-01-02"))
	expectedEnd := ui.StartDate.Add(24 * time.Hour).Format("2006-01-02")
	assert.Equal(t, expectedEnd, ui.EndDate.Format("2006-01-02"))

	chore := findEvent("chore")
	require.NotNil(t, chore)
	assert.Equal(t, "chore", chore.Type)
	assert.Equal(t, "chore", chore.Scope)
}

func TestRunTimeline_Integration(t *testing.T) {
	// Setup Mock
	mockGit := &MockGitClient{
		RepoExistsFunc: func(repoPath string) bool {
			return true
		},
		LogFunc: func(repoPath string, args ...string) ([]string, error) {
			return []string{
				"h1|2023-01-01T00:00:00Z|feat(api): init",
				"h2|2023-01-05T00:00:00Z|feat(api): finalize",
			}, nil
		},
	}

	// Override factory
	oldFactory := gitClientFactory
	gitClientFactory = func() IGitClient {
		return mockGit
	}
	defer func() { gitClientFactory = oldFactory }()

	// Reset flags and defer restoration
	oldDays := timelineDays
	oldOutput := timelineOutput
	oldFocus := timelineFocus
	defer func() {
		timelineDays = oldDays
		timelineOutput = oldOutput
		timelineFocus = oldFocus
	}()

	timelineDays = 30
	timelineOutput = "mermaid"
	timelineFocus = ""

	// Execute directly
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runTimeline(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()

	// Verify Output
	assert.Contains(t, output, "gantt")
	assert.Contains(t, output, "title Project Timeline")
	assert.Contains(t, output, "section api")
	assert.Contains(t, output, "2023-01-01")
	assert.Contains(t, output, "2023-01-05")
}

func TestRunTimeline_JSON(t *testing.T) {
	// Setup Mock
	mockGit := &MockGitClient{
		RepoExistsFunc: func(repoPath string) bool {
			return true
		},
		LogFunc: func(repoPath string, args ...string) ([]string, error) {
			return []string{
				"h1|2023-01-01T00:00:00Z|feat(core): something",
			}, nil
		},
	}

	// Override factory
	oldFactory := gitClientFactory
	gitClientFactory = func() IGitClient {
		return mockGit
	}
	defer func() { gitClientFactory = oldFactory }()

	// Set flags and defer restoration
	oldDays := timelineDays
	oldOutput := timelineOutput
	oldFocus := timelineFocus
	defer func() {
		timelineDays = oldDays
		timelineOutput = oldOutput
		timelineFocus = oldFocus
	}()

	timelineDays = 30
	timelineOutput = "json"
	timelineFocus = ""

	// Execute
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	err := runTimeline(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()

	// Verify Output
	assert.Contains(t, output, "\"scope\": \"core\"")
	assert.Contains(t, output, "\"type\": \"feat\"")
}
