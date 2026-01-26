package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestBreakingHelperProcess isn't a real test. It's used to mock exec.Command
func TestBreakingHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args[3:]
	if len(args) == 0 {
		return
	}

	cmd := args[0]
	cmdArgs := args[1:]

	switch cmd {
	case "git":
		if len(cmdArgs) > 0 {
			sub := cmdArgs[0]
			if sub == "ls-tree" {
				// Return a mock file list
				fmt.Println("mock_api.go")
			} else if sub == "show" {
				// Return mock content
				// Args: show base:path
				fmt.Printf("package main\nfunc RemovedFunc() {}\n")
			} else if sub == "rev-parse" {
				// Return mock repo root (current directory)
				cwd, _ := os.Getwd()
				fmt.Println(cwd)
			}
		}
	}
}

func mockBreakingExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestBreakingHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestBreakingCommand(t *testing.T) {
	// 1. Create a temporary file to analyze (Local)
	// We need to create it in the current working directory because filepath.Walk uses it
	// But running tests might be in a different dir?
	// Tests run in the package directory `cmd/recac`.
	// So we create "mock_api.go" there.
	f, err := os.Create("mock_api.go")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString("package main\nfunc NewFunc() {}\n")
	f.Close()
	defer os.Remove("mock_api.go")

	// 2. Mock execCommand
	oldExec := execCommand
	defer func() { execCommand = oldExec }()
	execCommand = mockBreakingExecCommand

	// 3. Execute Command
	// We expect: RemovedFunc removed, NewFunc added.
	// Note: breakingCmd is global
	// We need to reset flags just in case
	breakingBase = "main"
	breakingPath = "."
	breakingJSON = false
	breakingFail = false

	// Reset flags via executeCommand helper if it does so
	// executeCommand in test_helpers_test.go calls resetFlags(root)

	// We use executeCommand helper which captures output
	// But breakingCmd.RunE calls execCommand (the var)

	output, err := executeCommand(rootCmd, "breaking", "--base", "HEAD", "--path", ".")
	if err != nil {
		t.Fatalf("Command failed: %v", err)
	}

	if !strings.Contains(output, "REMOVED") {
		t.Errorf("Expected REMOVED in output, got:\n%s", output)
	}
	if !strings.Contains(output, "main.RemovedFunc") { // Fuzzy match because it might be ./main.RemovedFunc or just main.RemovedFunc depending on path logic
		t.Errorf("Expected main.RemovedFunc in output, got:\n%s", output)
	}

	if !strings.Contains(output, "ADDED") {
		t.Errorf("Expected ADDED in output, got:\n%s", output)
	}
	if !strings.Contains(output, "main.NewFunc") {
		t.Errorf("Expected main.NewFunc in output, got:\n%s", output)
	}
}
