package main

import (
	"context"
	"fmt"
	"os"
	"testing"

	"recac/internal/agent"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

// DiagnoseMockAgent is a simple mock for the Agent interface
type DiagnoseMockAgent struct {
	Response string
}

func (m *DiagnoseMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *DiagnoseMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestDiagnoseCmd_File(t *testing.T) {
	// 1. Create a dummy log file
	tmpFile, err := os.CreateTemp("", "error.log")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	content := "Fatal error: unexpected EOF\nAt line 42"
	_, err = tmpFile.WriteString(content)
	assert.NoError(t, err)
	tmpFile.Close()

	// 2. Setup Mock Agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockResponse := "Analysis: The file ended prematurely.\nFix: Close the brace."
	agentClientFactory = func(ctx context.Context, provider, model, workDir, project string) (agent.Agent, error) {
		return &DiagnoseMockAgent{Response: mockResponse}, nil
	}

	// Prevent interactive mode fallthrough
	origRun := rootCmd.Run
	rootCmd.Run = func(c *cobra.Command, args []string) {
		t.Fatal("rootCmd.Run (Interactive Mode) was called! Arguments matched no subcommand?")
	}
	defer func() { rootCmd.Run = origRun }()

	// 3. Execute Command using helper
	output, err := executeCommand(rootCmd, "diagnose", "--file", tmpFile.Name())
	assert.NoError(t, err)

	// 4. Verify Output
	assert.Contains(t, output, "Consulting Agent...")
	assert.Contains(t, output, mockResponse)
}

func TestDiagnoseCmd_Command(t *testing.T) {
	// 1. Setup Mock Agent
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	mockResponse := "Analysis: echo command worked."
	agentClientFactory = func(ctx context.Context, provider, model, workDir, project string) (agent.Agent, error) {
		return &DiagnoseMockAgent{Response: mockResponse}, nil
	}

	// Prevent interactive mode fallthrough
	origRun := rootCmd.Run
	rootCmd.Run = func(c *cobra.Command, args []string) {
		t.Fatal("rootCmd.Run (Interactive Mode) was called!")
	}
	defer func() { rootCmd.Run = origRun }()

	// 2. Execute Command
	// Note: We use "echo" which is safe.
	output, err := executeCommand(rootCmd, "diagnose", "--command", "echo hello world")
	assert.NoError(t, err)

	// 3. Verify Output
	assert.Contains(t, output, "Running command: echo hello world")
	assert.Contains(t, output, "Command executed successfully")
	assert.Contains(t, output, mockResponse)
}

func TestDiagnoseCmd_NoInput(t *testing.T) {
	// Prevent interactive mode fallthrough
	origRun := rootCmd.Run
	rootCmd.Run = func(c *cobra.Command, args []string) {
		t.Fatal("rootCmd.Run (Interactive Mode) was called!")
	}
	defer func() { rootCmd.Run = origRun }()

	// 1. Execute Command with no args
	output, err := executeCommand(rootCmd, "diagnose")

	// It should return an error
	assert.Error(t, err)
	// executeCommand captures stdout/stderr but if it returns err, output might be partial.
	// The error message comes from RunE returning error.

	// Wait, executeCommand helper in test_helpers_test.go:
	// If RunE returns error, root.Execute() returns error.
	// output = b.String() is returned.

	// But where is the error message printed?
	// Cobra prints error to stderr if SilenceErrors is false (it is true in rootCmd).
	// So Cobra returns error.
	// We check err.
	assert.Contains(t, err.Error(), "please provide")

	// Also check output for usage?
	fmt.Println("Output:", output)
}
