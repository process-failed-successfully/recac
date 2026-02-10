package main

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent for testing
type TestCmdMockAgent struct {
	mock.Mock
}

func (m *TestCmdMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *TestCmdMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func (m *TestCmdMockAgent) GetState() interface{} {
	return nil
}

func TestRunTest_ExplicitArgs(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Mock execCommand to capture 'go test'
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "go" && arg[0] == "test" {
			// Verify args
			expected := []string{"test", "-v", "pkg/a"}
			assert.Equal(t, expected, arg)
			return exec.Command("echo", "ok")
		}
		return exec.Command("echo", "unexpected")
	}
	defer func() { execCommand = exec.Command }()

	// Run
	output, err := executeCommand(cmd, "test", "pkg/a")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "Running tests for 1 packages")
	assert.Contains(t, output, "ok")
}

func TestRunTest_Impacted(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)

	// Mock execCommand for git diff (called by getGitDiffFiles if we didn't mock identifyPackagesFunc completely,
	// but we WILL mock identifyPackagesFunc to isolate the test logic)

	// Mock getGitDiffFilesFunc
	originalGetGitDiff := getGitDiffFilesFunc
	getGitDiffFilesFunc = func(staged bool) ([]string, error) {
		return []string{"file.go"}, nil
	}
	defer func() { getGitDiffFilesFunc = originalGetGitDiff }()

	// Mock identifyPackagesFunc
	originalIdentify := identifyPackagesFunc
	identifyPackagesFunc = func(files []string, root string) ([]string, map[string]bool, error) {
		return []string{"pkg/affected"}, map[string]bool{"pkg/changed": true}, nil
	}
	defer func() { identifyPackagesFunc = originalIdentify }()

	// Mock execCommand for go test
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "go" && arg[0] == "test" {
			// Verify args
			expected := []string{"test", "-v", "pkg/affected"}
			assert.Equal(t, expected, arg)
			return exec.Command("echo", "PASS")
		}
		return exec.Command("echo", "unexpected")
	}
	defer func() { execCommand = exec.Command }()

	// Run without args (triggers impact analysis)
	output, err := executeCommand(cmd, "test")

	// Assert
	assert.NoError(t, err)
	assert.Contains(t, output, "Analyzing impact")
	assert.Contains(t, output, "Running tests for 1 packages")
	assert.Contains(t, output, "PASS")
}

func TestRunTest_DiagnoseFailure(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)
	// Ensure diagnose is true (default, but explicit for safety)

	// Mock getGitDiffFilesFunc
	originalGetGitDiff := getGitDiffFilesFunc
	getGitDiffFilesFunc = func(staged bool) ([]string, error) {
		return []string{"fail.go"}, nil
	}
	defer func() { getGitDiffFilesFunc = originalGetGitDiff }()

	// Mock identifyPackagesFunc
	originalIdentify := identifyPackagesFunc
	identifyPackagesFunc = func(files []string, root string) ([]string, map[string]bool, error) {
		return []string{"pkg/fail"}, nil, nil
	}
	defer func() { identifyPackagesFunc = originalIdentify }()

	// Mock Agent
	mockAgent := new(TestCmdMockAgent)
	mockAgent.On("SendStream", mock.Anything, mock.Anything, mock.Anything).
		Return("Diagnosis: You broke it.", nil).
		Run(func(args mock.Arguments) {
			// Execute the callback to simulate streaming
			cb := args.Get(2).(func(string))
			cb("Diagnosis: You broke it.")
		})

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock execCommand to FAIL
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "go" {
			// Simulate failure
			// echo "FAIL" and exit 1
			c := exec.Command("sh", "-c", "echo 'FAIL output'; exit 1")
			return c
		}
		return exec.Command("echo", "unexpected")
	}
	defer func() { execCommand = exec.Command }()

	// Run
	// Note: executeCommand captures output but suppresses exit-1 from os.Exit.
	// But `runTest` returns error on failure. `executeCommand` helper handles return errors?
	// `executeCommand` in `test_helpers_test.go` calls `root.Execute()`.
	// If `RunE` returns error, `Execute` returns error.
	output, err := executeCommand(cmd, "test")

	// Assert
	// We expect an error because tests failed
	assert.Error(t, err)
	assert.Contains(t, output, "Tests failed")
	assert.Contains(t, output, "Diagnosing failure with AI")
	assert.Contains(t, output, "Diagnosis: You broke it")
}

func TestRunTest_Fix_Success(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)
	testFix = true // Enable fix mode
	// Ensure diagnose is false to avoid that path (or let it be true, but fix should return success first)
	// runTestOnce returns nil if fix succeeds.

	// Mock getGitDiff/identifyPackages
	originalGetGitDiff := getGitDiffFilesFunc
	getGitDiffFilesFunc = func(staged bool) ([]string, error) { return []string{"file.go"}, nil }
	defer func() { getGitDiffFilesFunc = originalGetGitDiff }()

	originalIdentify := identifyPackagesFunc
	identifyPackagesFunc = func(files []string, root string) ([]string, map[string]bool, error) {
		return []string{"pkg/fix"}, nil, nil
	}
	defer func() { identifyPackagesFunc = originalIdentify }()

	// Mock Agent
	mockAgent := new(TestCmdMockAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).
		Return("<file path=\"pkg/fix/fix.go\">fixed</file>", nil)

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock execCommand: First fail, then pass
	calls := 0
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "go" {
			calls++
			if calls == 1 {
				// Fail
				return exec.Command("sh", "-c", "echo 'FAIL output'; exit 1")
			}
			// Pass
			return exec.Command("echo", "PASS")
		}
		return exec.Command("echo", "unexpected")
	}
	defer func() { execCommand = exec.Command }()

	// Mock mkdirAllFunc
	originalMkdirAll := mkdirAllFunc
	mkdirAllFunc = func(path string, perm os.FileMode) error {
		return nil
	}
	defer func() { mkdirAllFunc = originalMkdirAll }()

	// Mock writeFileFunc
	originalWriteFile := writeFileFunc
	writeCalled := false
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		writeCalled = true
		assert.Equal(t, "pkg/fix/fix.go", name)
		assert.Equal(t, "fixed\n", string(data)) // ParseFileBlocks adds newline
		return nil
	}
	defer func() { writeFileFunc = originalWriteFile }()

	// Run
	output, err := executeCommand(cmd, "test")

	// Assert
	assert.NoError(t, err) // Should succeed eventually
	assert.Contains(t, output, "Attempting fix 1/3")
	assert.Contains(t, output, "Fix successful")
	assert.True(t, writeCalled)
}

func TestRunTest_Fix_GiveUp(t *testing.T) {
	// Setup
	cmd := rootCmd
	resetFlags(cmd)
	testFix = true
	testFixAttempts = 2

	// Mock getGitDiff/identifyPackages
	originalGetGitDiff := getGitDiffFilesFunc
	getGitDiffFilesFunc = func(staged bool) ([]string, error) { return []string{"file.go"}, nil }
	defer func() { getGitDiffFilesFunc = originalGetGitDiff }()

	originalIdentify := identifyPackagesFunc
	identifyPackagesFunc = func(files []string, root string) ([]string, map[string]bool, error) {
		return []string{"pkg/fix"}, nil, nil
	}
	defer func() { identifyPackagesFunc = originalIdentify }()

	// Mock Agent (return something so loop continues)
	mockAgent := new(TestCmdMockAgent)
	mockAgent.On("Send", mock.Anything, mock.Anything).
		Return("<file path=\"pkg/fix/fix.go\">fixed</file>", nil)

	originalAgentFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalAgentFactory }()

	// Mock execCommand: Always fail
	execCommand = func(name string, arg ...string) *exec.Cmd {
		if name == "go" {
			return exec.Command("sh", "-c", "echo 'FAIL output'; exit 1")
		}
		return exec.Command("echo", "unexpected")
	}
	defer func() { execCommand = exec.Command }()

	// Mock mkdirAllFunc
	originalMkdirAll := mkdirAllFunc
	mkdirAllFunc = func(path string, perm os.FileMode) error {
		return nil
	}
	defer func() { mkdirAllFunc = originalMkdirAll }()

	// Mock Write
	originalWriteFile := writeFileFunc
	writeFileFunc = func(name string, data []byte, perm os.FileMode) error {
		return nil
	}
	defer func() { writeFileFunc = originalWriteFile }()

	// Run
	output, err := executeCommand(cmd, "test")

	// Assert
	assert.Error(t, err)
	assert.Equal(t, "failed to fix tests after 2 attempts", err.Error())
	assert.Contains(t, output, "Attempting fix 1/2")
	assert.Contains(t, output, "Attempting fix 2/2")
}
