package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFuzzCodeBlock(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Markdown code block",
			input:    "Here is the code:\n```go\npackage main\nfunc FuzzTest(f *testing.F) {}\n```\nEnjoy!",
			expected: "package main\nfunc FuzzTest(f *testing.F) {}",
		},
		{
			name:     "Code block without language",
			input:    "```\npackage main\n```",
			expected: "package main",
		},
		{
			name:     "No code block",
			input:    "package main",
			expected: "package main",
		},
		{
			name:     "Empty input",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractFuzzCodeBlock(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// MockFuzzAgent implements Agent interface
type MockFuzzAgent struct {
	Response string
}

func (m *MockFuzzAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockFuzzAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestRunFuzz(t *testing.T) {
	// Setup temp file
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "lib.go")
	srcContent := `package lib

func Add(a, b int) int {
	return a + b
}
`
	err := os.WriteFile(srcFile, []byte(srcContent), 0644)
	require.NoError(t, err)

	// Mock agent
	mockAgent := &MockFuzzAgent{
		Response: "```go\npackage lib\nimport \"testing\"\nfunc FuzzAdd(f *testing.F) {\n\tf.Add(1, 2)\n\tf.Fuzz(func(t *testing.T, a, b int) {\n\t\tAdd(a, b)\n\t})\n}\n```",
	}

	// Override factories
	origAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, p, m, d, n string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origAgentFactory }()

	origExecCommand := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		// Mock go test execution
		// Just verify args
		if name == "go" && len(args) > 1 && args[0] == "test" {
			return exec.Command("echo", "fuzzing successful")
		}
		return exec.Command("echo", "unexpected command")
	}
	defer func() { execCommand = origExecCommand }()

	// Execute command
	// We need to change CWD because runFuzz uses os.Getwd()
	origWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(origWd)

	cmd := fuzzCmd
	// Reset flags
	fuzzFunc = ""
	fuzzDuration = "1s"
	fuzzKeep = false // Delete generated file

	err = runFuzz(cmd, []string{"lib.go"})

	require.NoError(t, err)

	// Verify file was deleted (fuzzKeep=false)
	fuzzFile := filepath.Join(tmpDir, "lib_fuzz_test.go")
	_, err = os.Stat(fuzzFile)
	assert.True(t, os.IsNotExist(err), "Fuzz file should be deleted")
}

func TestRunFuzz_NoFunction(t *testing.T) {
	tmpDir := t.TempDir()
	srcFile := filepath.Join(tmpDir, "empty.go")
	err := os.WriteFile(srcFile, []byte("package empty\n"), 0644)
	require.NoError(t, err)

	cmd := fuzzCmd
	err = runFuzz(cmd, []string{srcFile})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no exported functions found")
}
