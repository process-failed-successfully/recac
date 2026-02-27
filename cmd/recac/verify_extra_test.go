package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseDiffHunks(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		expected []LineInterval
	}{
		{
			name:     "Single addition",
			diff:     "@@ -1,2 +3,4 @@",
			expected: []LineInterval{{Start: 3, End: 6}}, // +3,4 means start 3, len 4 => 3,4,5,6
		},
		{
			name:     "Single deletion",
			diff:     "@@ -1,2 +3,0 @@",
			expected: nil, // 0 lines added
		},
		{
			name:     "Modification (add/del)",
			diff:     "@@ -10,5 +10,5 @@",
			expected: []LineInterval{{Start: 10, End: 14}},
		},
		{
			name:     "Multiple hunks",
			diff:     "@@ -1,2 +10,1 @@\n...\n@@ -5,2 +20,2 @@",
			expected: []LineInterval{{Start: 10, End: 10}, {Start: 20, End: 21}},
		},
		{
			name:     "Single line add (no comma)",
			diff:     "@@ -1 +10 @@",
			expected: []LineInterval{{Start: 10, End: 10}}, // Implicit count 1
		},
		{
			name:     "Empty",
			diff:     "",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDiffHunks(tt.diff)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsLineInIntervals(t *testing.T) {
	intervals := []LineInterval{
		{Start: 10, End: 20},
		{Start: 30, End: 30},
	}

	assert.True(t, isLineInIntervals(10, intervals))
	assert.True(t, isLineInIntervals(15, intervals))
	assert.True(t, isLineInIntervals(20, intervals))
	assert.True(t, isLineInIntervals(30, intervals))
	assert.False(t, isLineInIntervals(9, intervals))
	assert.False(t, isLineInIntervals(21, intervals))
	assert.False(t, isLineInIntervals(29, intervals))
}

func TestRunVerify_Integration(t *testing.T) {
	// Mock execCommand
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		// Handle git diff --name-only
		if len(args) >= 2 && args[0] == "diff" && args[1] == "--name-only" {
			// Return list of files
			return exec.Command("echo", "main.go\nutils.go")
		}
		// Handle git diff --unified=0
		if len(args) >= 2 && args[0] == "diff" && strings.HasPrefix(args[1], "--unified") {
			// Return dummy diff
			return exec.Command("echo", "@@ -1,0 +10,2 @@\n+func New() {}")
		}
		return exec.Command("false")
	}

	cmd := &cobra.Command{}
	// Run
	err := runVerify(cmd, []string{})

	// Since we mock security/complexity scans implicitly by files not being real or empty,
	// runVerify likely returns nil (no issues found).
	// We just want to ensure it runs through the logic without crashing.
	require.NoError(t, err)
}

func TestRunVerify_NoChangedFiles(t *testing.T) {
	// Mock execCommand
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		// Return empty
		return exec.Command("echo", "")
	}

	cmd := &cobra.Command{}
	err := runVerify(cmd, []string{})
	require.NoError(t, err)
}

func TestPrintVerifyTable(t *testing.T) {
	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	issues := []VerifyIssue{
		{
			Severity: "High",
			Type:     "Security",
			File:     "main.go",
			Line:     10,
			Message:  "Bad thing",
		},
		{
			Severity: "Medium",
			Type:     "Complexity",
			File:     "utils.go",
			Line:     5,
			Message:  "Complex thing",
		},
	}

	printVerifyTable(cmd, issues)

	output := buf.String()
	assert.Contains(t, output, "SEVERITY")
	assert.Contains(t, output, "🔴 High")
	assert.Contains(t, output, "Bad thing")
	assert.Contains(t, output, "🟡 Medium")
	assert.Contains(t, output, "Complex thing")
}

func TestRunVerify_JSON(t *testing.T) {
	// Setup flags
	verifyJSON = true
	defer func() { verifyJSON = false }()

	// Mock execCommand
	origExecCommand := execCommand
	defer func() { execCommand = origExecCommand }()

	execCommand = func(name string, args ...string) *exec.Cmd {
		// Return no files to keep it simple but hit the early return
		return exec.Command("echo", "")
	}

	cmd := &cobra.Command{}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := runVerify(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.JSONEq(t, "[]", output)
}
