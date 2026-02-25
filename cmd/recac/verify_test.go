package main

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestParseDiffHunks(t *testing.T) {
	tests := []struct {
		name     string
		diff     string
		expected []LineInterval
	}{
		{
			name: "Simple hunk",
			diff: `@@ -1,2 +3,4 @@
 line 1
 line 2`,
			expected: []LineInterval{{Start: 3, End: 6}},
		},
		{
			name: "Multiple hunks",
			diff: `@@ -1,2 +10,2 @@
 line 1
 line 2
@@ -5,2 +20,1 @@
 line 3`,
			expected: []LineInterval{{Start: 10, End: 11}, {Start: 20, End: 20}},
		},
		{
			name: "Zero lines added",
			diff: `@@ -1,2 +10,0 @@`,
			expected: nil,
		},
		{
			name: "Single line added (implicit count 1)",
			diff: `@@ -1,2 +5 @@`,
			expected: []LineInterval{{Start: 5, End: 5}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			intervals := parseDiffHunks(tt.diff)
			assert.Equal(t, tt.expected, intervals)
		})
	}
}

func TestIsLineInIntervals(t *testing.T) {
	intervals := []LineInterval{
		{Start: 5, End: 10},
		{Start: 20, End: 25},
	}

	assert.True(t, isLineInIntervals(5, intervals))
	assert.True(t, isLineInIntervals(10, intervals))
	assert.True(t, isLineInIntervals(7, intervals))
	assert.True(t, isLineInIntervals(20, intervals))
	assert.True(t, isLineInIntervals(25, intervals))

	assert.False(t, isLineInIntervals(4, intervals))
	assert.False(t, isLineInIntervals(11, intervals))
	assert.False(t, isLineInIntervals(19, intervals))
	assert.False(t, isLineInIntervals(26, intervals))
}

func TestPrintVerifyTable(t *testing.T) {
	issues := []VerifyIssue{
		{
			File:     "main.go",
			Line:     10,
			Type:     "Complexity",
			Message:  "Too complex",
			Severity: "High",
		},
		{
			File:     "utils.go",
			Line:     5,
			Type:     "Security",
			Message:  "Hardcoded secret",
			Severity: "Medium",
		},
	}

	var buf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&buf)

	printVerifyTable(cmd, issues)

	output := buf.String()
	assert.Contains(t, output, "SEVERITY")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "FILE")
	assert.Contains(t, output, "LINE")
	assert.Contains(t, output, "MESSAGE")

	assert.Contains(t, output, "🔴 High")
	assert.Contains(t, output, "Complexity")
	assert.Contains(t, output, "main.go")
	assert.Contains(t, output, "10")
	assert.Contains(t, output, "Too complex")

	assert.Contains(t, output, "🟡 Medium")
	assert.Contains(t, output, "Security")
	assert.Contains(t, output, "utils.go")
	assert.Contains(t, output, "5")
	assert.Contains(t, output, "Hardcoded secret")
}
