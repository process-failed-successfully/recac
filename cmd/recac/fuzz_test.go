package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type FuzzTestMockAgent struct {
	Response string
}

func (m *FuzzTestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *FuzzTestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return "", nil
}

func TestFuzzCmd(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "calculator.go")
	content := `package calc

func Add(a, b int) int {
	return a + b
}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	// Mock Agent
	mockAgent := &FuzzTestMockAgent{
		Response: "```go\npackage calc\n\nimport \"testing\"\n\nfunc FuzzAdd(f *testing.F) {\n\tf.Add(1, 2)\n\tf.Fuzz(func(t *testing.T, a, b int) {\n\t\tAdd(a, b)\n\t})\n}\n```",
	}

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock execCommand
	originalExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		// Verify it's the go test command
		if name == "go" && len(arg) > 0 && arg[0] == "test" {
			// Check if we are running the helper process
			// We can't check arguments easily because they are flags
			// But we know runFuzz calls it with -fuzz

			// Call the helper process
			exe, _ := os.Executable()
			cmd := exec.Command(exe, "-test.run=TestHelperProcess_FuzzSuccess", "--")
			cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
			return cmd
		}
		return originalExecCommand(name, arg...)
	}
	defer func() { execCommand = originalExecCommand }()

	// Run Command
	// We need to set the global flags manually because we are calling runFuzz directly
	// and not via Execute() which parses args.
	fuzzFunc = "Add"
	fuzzDuration = "10ms"
	fuzzKeep = true

	// Also update the flags on the command object just in case
	fuzzCmd.Flags().Set("func", "Add")
	fuzzCmd.Flags().Set("duration", "10ms")
	fuzzCmd.Flags().Set("keep", "true")

	ctx := context.Background()
	fuzzCmd.SetContext(ctx)

	// Redirect output to avoid clutter
	// fuzzCmd.SetOut(io.Discard)
	// fuzzCmd.SetErr(io.Discard)

	err = runFuzz(fuzzCmd, []string{filePath})
	assert.NoError(t, err)

	// Verify file created
	fuzzFile := filepath.Join(tmpDir, "calculator_fuzz_test.go")
	assert.FileExists(t, fuzzFile)

	fuzzContent, _ := os.ReadFile(fuzzFile)
	assert.Contains(t, string(fuzzContent), "func FuzzAdd")
}

// TestHelperProcess_FuzzSuccess acts as the "go test -fuzz" process
func TestHelperProcess_FuzzSuccess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// Simulate success output
	fmt.Println("PASS")
	fmt.Println("ok  \tcommand-line-arguments\t0.010s")
	os.Exit(0)
}

func TestFuzzCmd_NoFunc(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "simple.go")
	content := `package simple
func Exported() {}
`
	err := os.WriteFile(filePath, []byte(content), 0644)
	require.NoError(t, err)

	// Mock Agent
	mockAgent := &FuzzTestMockAgent{
		Response: "```go\npackage simple\nfunc FuzzExported(f *testing.F) {}\n```",
	}
	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock Exec
	originalExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		exe, _ := os.Executable()
		cmd := exec.Command(exe, "-test.run=TestHelperProcess_FuzzSuccess", "--")
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() { execCommand = originalExecCommand }()

	// Reset globals
	fuzzFunc = ""
	fuzzDuration = "10ms"
	fuzzKeep = true

	fuzzCmd.Flags().Set("func", "")

	ctx := context.Background()
	fuzzCmd.SetContext(ctx)

	err = runFuzz(fuzzCmd, []string{filePath})
	assert.NoError(t, err)
}
