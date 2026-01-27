package main

import (
	"fmt"
	"os"
	"strings"
	"testing"
)

func TestBreakingCommand(t *testing.T) {
	// 1. Create a temporary file to analyze (Local)
	// We need to create it in the current working directory because filepath.Walk uses it
	f, err := os.Create("mock_api.go")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("package main\nfunc NewFunc() {}\n")
	f.Close()
	defer os.Remove("mock_api.go")

	// 2. Mock gitClientFactory
	oldFactory := gitClientFactory
	defer func() { gitClientFactory = oldFactory }()

	mockGit := &MockGitClient{
		RunFunc: func(repoPath string, args ...string) (string, error) {
			if len(args) == 0 {
				return "", fmt.Errorf("no args")
			}
			switch args[0] {
			case "rev-parse":
				// git rev-parse --show-toplevel
				cwd, _ := os.Getwd()
				return cwd, nil
			case "ls-tree":
				// git ls-tree -r --name-only <base> <path>
				return "mock_api.go", nil
			case "show":
				// git show <base>:<path>
				return "package main\nfunc RemovedFunc() {}\n", nil
			default:
				return "", fmt.Errorf("unexpected command: %v", args)
			}
		},
	}
	gitClientFactory = func() IGitClient {
		return mockGit
	}

	// 3. Execute Command
	// Note: breakingCmd is global
	// We need to reset flags just in case
	breakingBase = "main"
	breakingPath = "."
	breakingJSON = false
	breakingFail = false

	// executeCommand helper resets flags, but breakingBase/Path are vars bound to flags,
	// so resetFlags might not reset these vars if they are bound via StringVar (pointers).
	// But executeCommand calls resetFlags(rootCmd).
	// resetFlags function in test_helpers_test.go resets flag values.
	// Since StringVar takes a pointer, resetting the flag value via pflag should update the variable.
	// However, let's just rely on passing args to executeCommand which sets them.

	output, err := executeCommand(rootCmd, "breaking", "--base", "HEAD", "--path", ".")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !strings.Contains(output, "REMOVED") {
		t.Errorf("Expected REMOVED in output, got:\n%s", output)
	}
	if !strings.Contains(output, "main.RemovedFunc") {
		t.Errorf("Expected main.RemovedFunc in output, got:\n%s", output)
	}

	if !strings.Contains(output, "ADDED") {
		t.Errorf("Expected ADDED in output, got:\n%s", output)
	}
	if !strings.Contains(output, "main.NewFunc") {
		t.Errorf("Expected main.NewFunc in output, got:\n%s", output)
	}
}
