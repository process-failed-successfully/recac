package main

import (
	"context"
	"fmt"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCoverageCommand_ParseAndAnalyze(t *testing.T) {
	// Setup mocks
	originalTestsFunc := runTestsFunc
	originalCoverToolFunc := runCoverToolFunc
	originalAgentFactory := agentClientFactory
	defer func() {
		runTestsFunc = originalTestsFunc
		runCoverToolFunc = originalCoverToolFunc
		agentClientFactory = originalAgentFactory
	}()

	mockAgent := new(MockAgent) // Reusing from tickets_test.go if in same package
	// If MockAgent is not available (e.g. if tickets_test.go is ignored or different build tags), we might fail.
	// But assuming standard package test.

	// Mock runTestsFunc to simulate success
	runTestsFunc = func(args []string) error {
		// Just simulate success
		return nil
	}

	// Mock runCoverToolFunc to return fake coverage data
	runCoverToolFunc = func(args []string) ([]byte, error) {
		output := `
github.com/user/repo/pkg/auth.go:10:	Login		50.0%
github.com/user/repo/pkg/utils.go:20:	Helper		100.0%
github.com/user/repo/pkg/core.go:30:	Process		40.0%
total:									(statements)	60.0%
`
		return []byte(output), nil
	}

	// Mock agent interaction
	agentClientFactory = func(ctx context.Context, provider, model, dir, usage string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Expectation for AI
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			onChunk := args.Get(2).(func(string))
			onChunk("1. Login: Critical for security.\n2. Process: Core business logic.")
		}).
		Return("", nil)

	// Execute command with flags
	cmd, out, _ := newRootCmd()
	cmd.SetArgs([]string{"coverage", "--analyze", "--threshold=80"})

	err := cmd.Execute()
	assert.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "Functions below 80.0% coverage:")
	assert.Contains(t, output, "Login")
	assert.Contains(t, output, "Process")
	assert.NotContains(t, output, "Helper") // Should be filtered out
	assert.Contains(t, output, "1. Login: Critical for security.")

	mockAgent.AssertExpectations(t)
}

func TestCoverageCommand_NoGaps(t *testing.T) {
	// Setup mocks
	originalTestsFunc := runTestsFunc
	originalCoverToolFunc := runCoverToolFunc
	defer func() {
		runTestsFunc = originalTestsFunc
		runCoverToolFunc = originalCoverToolFunc
	}()

	runTestsFunc = func(args []string) error { return nil }
	runCoverToolFunc = func(args []string) ([]byte, error) {
		return []byte(`
github.com/user/repo/pkg/file.go:10:	Func		90.0%
total:									(statements)	90.0%
`), nil
	}

	cmd, out, _ := newRootCmd()
	cmd.SetArgs([]string{"coverage", "--threshold=80"})

	err := cmd.Execute()
	assert.NoError(t, err)

	assert.Contains(t, out.String(), "All functions meet the coverage threshold")
}

func TestCoverageCommand_TestFailure(t *testing.T) {
	originalTestsFunc := runTestsFunc
	defer func() { runTestsFunc = originalTestsFunc }()

	runTestsFunc = func(args []string) error {
		return fmt.Errorf("compilation failed")
	}

	cmd, _, _ := newRootCmd()
	cmd.SetArgs([]string{"coverage"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tests failed")
}
