package main

import (
	"context"
	"fmt"
	"recac/internal/cmdutils"
	"recac/internal/jira"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 10, "hello w..."},
		{"abc", 3, "abc"},
		{"abcd", 3, "..."},
	}

	for _, tt := range tests {
		actual := truncateString(tt.input, tt.maxLen)
		assert.Equal(t, tt.expected, actual, "truncateString(%q, %d)", tt.input, tt.maxLen)
	}
}

func TestJiraCleanupCmd_NoLabel(t *testing.T) {
	cmd := &cobra.Command{Use: "cleanup"}
	cmd.Flags().String("label", "", "")

	var exitCode int
	originalExit := exit
	exit = func(code int) {
		exitCode = code
		panic("exit")
	}
	defer func() { exit = originalExit }()

	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, 1, exitCode)
		}
	}()

	jiraCleanupCmd.Run(cmd, []string{})
}

func TestJiraCleanupCmd_ClientError(t *testing.T) {
	cmd := &cobra.Command{Use: "cleanup"}
	cmd.Flags().String("label", "test-label", "")
	cmd.Flags().Set("label", "test-label")

	originalGetJiraClient := cmdutils.GetJiraClient
	cmdutils.GetJiraClient = func(ctx context.Context) (*jira.Client, error) {
		return nil, fmt.Errorf("mock client error")
	}
	defer func() { cmdutils.GetJiraClient = originalGetJiraClient }()

	var exitCode int
	originalExit := exit
	exit = func(code int) {
		exitCode = code
		panic("exit")
	}
	defer func() { exit = originalExit }()

	defer func() {
		if r := recover(); r != nil {
			assert.Equal(t, 1, exitCode)
		}
	}()

	jiraCleanupCmd.Run(cmd, []string{})
}
