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

	"github.com/AlecAivazis/survey/v2"
	"github.com/spf13/viper"
)

// Mock for Agent
type MockUpgradeAgent struct {
	Response string
}

func (m *MockUpgradeAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockUpgradeAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

// TestUpgradeHelperProcess is used to mock exec.Command
func TestUpgradeHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	args := os.Args
	for len(args) > 0 {
		if args[0] == "--" {
			args = args[1:]
			break
		}
		args = args[1:]
	}
	if len(args) == 0 {
		fmt.Fprintf(os.Stderr, "No command\n")
		os.Exit(2)
	}

	cmd, subcmd := args[0], args[1:]
	switch cmd {
	case "go":
		if len(subcmd) > 0 && subcmd[0] == "list" {
			// Mock go list output
			// path, version, update
			fmt.Print(`
{
	"Path": "github.com/example/lib",
	"Version": "v1.0.0",
	"Update": {
		"Version": "v1.1.0"
	}
}
`)
			os.Exit(0)
		}
		if len(subcmd) > 0 && subcmd[0] == "get" {
			// Mock go get success
			os.Exit(0)
		}
		if len(subcmd) > 0 && subcmd[0] == "test" {
			// Mock go test failure if requested
			if os.Getenv("MOCK_TEST_FAIL") == "1" {
				fmt.Println("FAIL: TestExample")
				os.Exit(1)
			}
			fmt.Println("PASS")
			os.Exit(0)
		}
	case "npm":
		if len(subcmd) > 0 && subcmd[0] == "outdated" {
			// Mock npm outdated output
			fmt.Print(`
{
  "react": {
    "current": "17.0.0",
    "latest": "18.0.0"
  }
}
`)
			os.Exit(0)
		}
	case "git":
		if len(subcmd) > 0 && subcmd[0] == "diff" {
			fmt.Println("diff content")
			os.Exit(0)
		}
	}
	os.Exit(0)
}

func TestRunUpgrade_Flow(t *testing.T) {
	// 1. Setup Environment
	tempDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)

	// Create dummy go.mod
	os.WriteFile("go.mod", []byte("module test"), 0644)

	// 2. Mock execCommand
	originalExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestUpgradeHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() { execCommand = originalExecCommand }()

	// 3. Mock executeShellCommand (used for go test)
	originalExecuteShell := executeShellCommand
	executeShellCommand = func(command string) (string, error) {
		// Just reuse execCommand logic via helper
		parts := strings.Fields(command)
		cmd := execCommand(parts[0], parts[1:]...)
		// Inject failure if needed
		if strings.Contains(command, "go test") && os.Getenv("TEST_FAIL_ONCE") == "1" {
			cmd.Env = append(cmd.Env, "MOCK_TEST_FAIL=1")
			os.Setenv("TEST_FAIL_ONCE", "0") // Reset for next retry
			// But environmental variable inheritance in helper process might be tricky.
			// Actually executeShellCommand returns combined output and error.
			out, err := cmd.CombinedOutput()
			// If we want to simulate failure, we need to ensure the helper exits with non-zero.
			// The helper checks MOCK_TEST_FAIL.
			return string(out), err
		}
		out, _ := cmd.CombinedOutput()
		return string(out), nil
	}
	defer func() { executeShellCommand = originalExecuteShell }()

	// 4. Mock askOneFunc (Survey)
	originalAskOne := askOneFunc
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		// Mock selecting all
		// response is *[]string for MultiSelect
		if ptr, ok := response.(*[]string); ok {
			*ptr = []string{"[GO] github.com/example/lib (v1.0.0 -> v1.1.0)"}
		}
		return nil
	}
	defer func() { askOneFunc = originalAskOne }()

	// 5. Mock Agent
	originalFactory := agentClientFactory
	mockAgent := &MockUpgradeAgent{
		Response: `<file path="fixed.go">package main</file>`,
	}
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// 6. Run Upgrade (Success Case)
	viper.Set("provider", "mock")
	viper.Set("model", "mock")

	cmd := upgradeCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := runUpgrade(cmd, []string{})
	if err != nil {
		t.Fatalf("runUpgrade failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Updated github.com/example/lib") {
		t.Errorf("Expected update message, got: %s", output)
	}
	if !strings.Contains(output, "Upgrade complete") {
		t.Errorf("Expected completion message, got: %s", output)
	}
}

func TestRunUpgrade_FixLoop(t *testing.T) {
	// Similar setup but trigger a test failure
	tempDir := t.TempDir()
	cwd, _ := os.Getwd()
	os.Chdir(tempDir)
	defer os.Chdir(cwd)
	os.WriteFile("go.mod", []byte("module test"), 0644)

	// Mock execCommand
	originalExecCommand := execCommand
	execCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestUpgradeHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
		return cmd
	}
	defer func() { execCommand = originalExecCommand }()

	// Mock executeShellCommand
	originalExecuteShell := executeShellCommand
	callCount := 0
	executeShellCommand = func(command string) (string, error) {
		callCount++
		// First call (test) -> Fail
		// Note: Use strings.Contains because actual command is "go test ./..."
		if strings.Contains(command, "go test") && callCount == 1 {
			return "FAIL: TestExample", fmt.Errorf("exit status 1")
		}
		// Git diff call -> empty
		if strings.Contains(command, "git diff") {
			return "", nil
		}
		// Second call (test after fix) -> Pass
		return "PASS", nil
	}
	defer func() { executeShellCommand = originalExecuteShell }()

	// Mock Survey
	originalAskOne := askOneFunc
	askOneFunc = func(p survey.Prompt, response interface{}, opts ...survey.AskOpt) error {
		if ptr, ok := response.(*[]string); ok {
			*ptr = []string{"[GO] github.com/example/lib (v1.0.0 -> v1.1.0)"}
		}
		return nil
	}
	defer func() { askOneFunc = originalAskOne }()

	// Mock Agent
	mockAgent := &MockUpgradeAgent{
		Response: `<file path="fixed.go">package main</file>`,
	}
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	cmd := upgradeCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	err := runUpgrade(cmd, []string{})
	if err != nil {
		t.Fatalf("runUpgrade failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Tests failed. Asking AI to fix") {
		t.Errorf("Expected fix attempt, got: %s", output)
	}
	if !strings.Contains(output, "Fixed fixed.go") {
		t.Errorf("Expected fix application, got: %s", output)
	}

	// Verify fixed file exists
	if _, err := os.Stat(filepath.Join(tempDir, "fixed.go")); os.IsNotExist(err) {
		t.Errorf("Fixed file was not created")
	}
}
