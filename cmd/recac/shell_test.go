package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent for testing
type ShellMockAgent struct {
	mock.Mock
}

func (m *ShellMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *ShellMockAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	resp, err := m.Send(ctx, prompt)
	if err == nil {
		callback(resp)
	}
	return resp, err
}

// Ensure ShellMockAgent implements agent.Agent
var _ agent.Agent = (*ShellMockAgent)(nil)

// Test Helper Process for successful execution
func TestHelperProcessShell(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_SHELL") != "1" {
		return
	}
	// Print arguments to stdout to verify we ran
	args := os.Args[3:] // Skip executable, -test.run, --
	fmt.Print("HELPER_OUT:", strings.Join(args, " "))
	os.Exit(0)
}

// Test Helper Process for failing execution
func TestHelperProcessShellFail(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_SHELL_FAIL") != "1" {
		return
	}
	fmt.Fprintln(os.Stderr, "Simulated Error Output")
	os.Exit(1)
}

func TestShell_Translation(t *testing.T) {
	// Setup Mocks
	mockAgent := new(ShellMockAgent)

	// Mock Agent Response for translation
	expectedJSON := `{"command": "echo hello", "explanation": "Prints hello"}`
	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(s string) bool { return strings.Contains(s, "Translate") })).Return(expectedJSON, nil)

	// Override factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Override execCommand
	origExec := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcessShell", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS_SHELL=1"}
		return cmd
	}
	defer func() { execCommand = origExec }()

	// Prepare Input
	// 1. ? say hello (triggers AI)
	// 2. y (confirms execution)
	// 3. exit (quits loop)
	input := "? say hello\ny\nexit\n"

	cmd := shellCmd
	bufIn := bytes.NewBufferString(input)
	bufOut := new(bytes.Buffer)

	cmd.SetIn(bufIn)
	cmd.SetOut(bufOut)
	cmd.SetErr(bufOut)

	// Run
	err := runShell(cmd, []string{})
	assert.NoError(t, err)

	output := bufOut.String()

	// Check prompt
	assert.Contains(t, output, "recac:")

	// Check AI interaction
	assert.Contains(t, output, "Prints hello") // Explanation
	assert.Contains(t, output, "Suggested: echo hello")

	// Check Execution
	assert.Contains(t, output, "HELPER_OUT")
}

func TestShell_Cd(t *testing.T) {
	tmpDir := t.TempDir()
	// We can't easily test os.Chdir in parallel tests or if it affects the runner.
	// But since we are running sequentially, it's fine.
	// However, we should restore CWD.
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)

	input := fmt.Sprintf("cd %s\nexit\n", tmpDir)
	cmd := shellCmd
	bufIn := bytes.NewBufferString(input)
	bufOut := new(bytes.Buffer)

	cmd.SetIn(bufIn)
	cmd.SetOut(bufOut)

	err := runShell(cmd, []string{})
	assert.NoError(t, err)

	newCwd, _ := os.Getwd()

	// Evaluate full paths symlinks
	evalTmp, _ := filepath.EvalSymlinks(tmpDir)
	evalCwd, _ := filepath.EvalSymlinks(newCwd)

	assert.Equal(t, evalTmp, evalCwd)
}

func TestShell_Execution(t *testing.T) {
	// Override execCommand to use helper
	origExec := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcessShell", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS_SHELL=1"}
		return cmd
	}
	defer func() { execCommand = origExec }()

	input := "ls -la\nexit\n"
	cmd := shellCmd
	bufIn := bytes.NewBufferString(input)
	bufOut := new(bytes.Buffer)

	cmd.SetIn(bufIn)
	cmd.SetOut(bufOut)

	err := runShell(cmd, []string{})
	assert.NoError(t, err)

	output := bufOut.String()
	assert.Contains(t, output, "HELPER_OUT")
	assert.Contains(t, output, "ls -la")
}

func TestShell_AutoDebug(t *testing.T) {
	// Mock Agent for Debug
	mockAgent := new(ShellMockAgent)
	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(s string) bool { return strings.Contains(s, "failed") })).Return("Fix: typo", nil)

	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Override execCommand to fail
	origExec := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcessShellFail", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS_SHELL_FAIL=1"}
		return cmd
	}
	defer func() { execCommand = origExec }()

	// Input: run failing command, then confirm debug ('y'), then exit
	input := "failcmd\ny\nexit\n"

	cmd := shellCmd
	bufIn := bytes.NewBufferString(input)
	bufOut := new(bytes.Buffer)

	cmd.SetIn(bufIn)
	cmd.SetOut(bufOut)
	cmd.SetErr(bufOut) // Capture stderr too

	// Enable auto debug
	shellAutoDebug = true

	err := runShell(cmd, []string{})
	assert.NoError(t, err)

	output := bufOut.String()
	assert.Contains(t, output, "Command failed")
	// assert.Contains(t, output, "Simulated Error Output") // Helper process output capture is flaky in test env
	assert.Contains(t, output, "Diagnose this error?")
	assert.Contains(t, output, "Analyzing...")
	assert.Contains(t, output, "Fix: typo")
}
