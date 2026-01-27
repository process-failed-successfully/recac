package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestDevHelperProcess is used to mock exec.Command
func TestDevHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	// Simulate success
	os.Exit(0)
}

func fakeExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestDevHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
	return cmd
}

func TestDevCmd(t *testing.T) {
	// 1. Setup Temp Dir
	tempDir, err := os.MkdirTemp("", "recac-dev-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create dummy go.mod to trigger detection
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module test"), 0644); err != nil {
		t.Fatalf("Failed to create go.mod: %v", err)
	}

	// 2. Mock devExecCommand
	var executedCommands []string
	var mu sync.Mutex

	originalExec := devExecCommand
	defer func() { devExecCommand = originalExec }()

	devExecCommand = func(command string, args ...string) *exec.Cmd {
		mu.Lock()
		executedCommands = append(executedCommands, command+" "+strings.Join(args, " "))
		mu.Unlock()
		return fakeExecCommand(command, args...)
	}

	// 3. Configure Command
	// We need to reset flags because they are package-level globals in dev.go
	devWatchDir = tempDir
	devCmdFlag = ""
	devExtensions = ""
	devRecursive = false
	devDebounce = 100 * time.Millisecond // Short debounce for test

	// 4. Run Dev Loop in Goroutine
	// We can't easily stop it, so we'll just let it leak or we need to refactor dev.go to be cancellable.
	// For this test, leaking one goroutine is acceptable, or we can use a context if we modify dev.go.
	// But let's verify logic first.

	// Note: runDev blocks. We run it in a goroutine.
	go func() {
		// Suppress stdout for clean test output
		// devCmd.SetOut(io.Discard)
		// devCmd.SetErr(io.Discard)
		// Actually runDev uses fmt.Printf / os.Stdout directly in some places (bad practice but common in CLIs)
		// So we can't easily suppress all output without capturing stdout/stderr of the process,
		// but since we are in a test binary, we can just let it print.
		runDev(devCmd, []string{})
	}()

	// Wait for watcher to start (heuristic)
	time.Sleep(500 * time.Millisecond)

	// 5. Trigger File Change
	testFile := filepath.Join(tempDir, "main.go")
	if err := os.WriteFile(testFile, []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	// Wait for debounce + execution
	time.Sleep(1000 * time.Millisecond)

	// 6. Assert
	mu.Lock()
	count := len(executedCommands)
	mu.Unlock()

	// Should be at least 2: 1 for initial run, 1 for file change
	// In runDev, we have:
	// go func() { trigger <- struct{}{} }() // Initial run
	// And then the event trigger.

	assert.GreaterOrEqual(t, count, 2, "Should execute command at least twice (init + change)")

	mu.Lock()
	if len(executedCommands) > 0 {
		assert.Contains(t, executedCommands[0], "go test ./...", "Should execute auto-detected command")
	}
	mu.Unlock()
}

func TestDevCmd_Manual(t *testing.T) {
	// Test with manual command flag
	tempDir, err := os.MkdirTemp("", "recac-dev-manual-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	var executedCommands []string
	var mu sync.Mutex

	originalExec := devExecCommand
	defer func() { devExecCommand = originalExec }()

	devExecCommand = func(command string, args ...string) *exec.Cmd {
		mu.Lock()
		executedCommands = append(executedCommands, command+" "+strings.Join(args, " "))
		mu.Unlock()
		return fakeExecCommand(command, args...)
	}

	devWatchDir = tempDir
	devCmdFlag = "echo manual"
	devExtensions = ".txt"
	devRecursive = false
	devDebounce = 100 * time.Millisecond

	go func() {
		runDev(devCmd, []string{})
	}()

	time.Sleep(500 * time.Millisecond)

	testFile := filepath.Join(tempDir, "test.txt")
	os.WriteFile(testFile, []byte("hello"), 0644)

	time.Sleep(1000 * time.Millisecond)

	mu.Lock()
	count := len(executedCommands)
	lastCmd := ""
	if count > 0 {
		lastCmd = executedCommands[len(executedCommands)-1]
	}
	mu.Unlock()

	assert.GreaterOrEqual(t, count, 2)
	assert.Contains(t, lastCmd, "echo manual")
}
