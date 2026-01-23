package git

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// TestHelperProcess isn't a real test. It's used as a helper process
// for exec.Command mocks.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	// Read command args
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

	cmd, args := args[0], args[1:]
	switch cmd {
	case "gh":
		// Mock GitHub CLI
		if len(args) >= 2 && args[0] == "pr" && args[1] == "create" {
			// Check for fail flag
			if os.Getenv("MOCK_GH_FAIL") == "true" {
				fmt.Fprintf(os.Stderr, "failed to create PR\n")
				os.Exit(1)
			}
			fmt.Println("https://github.com/owner/repo/pull/123")
		}
	case "git":
		if len(args) > 0 {
			subCmd := args[0]
			switch subCmd {
			case "bisect":
				// Mock bisect commands
				if len(args) > 1 {
					switch args[1] {
					case "start":
						fmt.Println("Bisecting: 6 revisions left to test after this (roughly 3 steps)")
					case "good":
						fmt.Println("Bisecting: 3 revisions left to test after this (roughly 2 steps)")
					case "bad":
						fmt.Println("Bisecting: 1 revision left to test after this (roughly 1 step)")
					case "reset":
						fmt.Println("Already on 'master'")
					case "log":
						fmt.Println("# bad: [abc1234] bad commit")
						fmt.Println("# good: [def5678] good commit")
						fmt.Println("git bisect start 'abc1234' 'def5678'")
						fmt.Println("git bisect bad abc1234")
						fmt.Println("git bisect good def5678")
					}
				}
			case "diff":
				if os.Getenv("MOCK_GIT_FAIL") == "true" {
					fmt.Fprintf(os.Stderr, "git diff failed\n")
					os.Exit(1)
				}
				if len(args) > 1 && args[1] == "--stat" {
					// Check for specific exit code simulation
					if os.Getenv("MOCK_GIT_EXIT_CODE") != "" {
						os.Exit(1) // Simulate exit code 1
					}
					fmt.Println(" file1 | 1 +")
					fmt.Println(" 1 file changed, 1 insertion(+)")
				} else {
					fmt.Println("diff --git a/file1 b/file1")
					fmt.Println("index 0000000..1111111 100644")
				}
			case "log":
				if os.Getenv("MOCK_GIT_FAIL") == "true" {
					fmt.Fprintf(os.Stderr, "git log failed\n")
					os.Exit(1)
				}
				fmt.Println("commit 123456")
				fmt.Println("Author: Me")
				fmt.Println("Date: Now")
				fmt.Println()
				fmt.Println("    message")
			}
		}
	}
}

// mockExecCommand mocks exec.Command using the helper process.
func mockExecCommand(command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

// mockExecCommandContext mocks exec.CommandContext.
func mockExecCommandContext(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS=1")
	return cmd
}

func TestClient_CreatePR(t *testing.T) {
	// Restore original execCommand after test
	defer func() {
		execCommand = exec.Command
		execCommandContext = exec.CommandContext
	}()
	execCommand = mockExecCommand
	execCommandContext = mockExecCommandContext

	c := NewClient()

	// Test Success
	url, err := c.CreatePR(".", "Title", "Body", "main")
	if err != nil {
		t.Errorf("CreatePR failed: %v", err)
	}
	if url != "https://github.com/owner/repo/pull/123" {
		t.Errorf("Expected URL 'https://github.com/owner/repo/pull/123', got '%s'", url)
	}

	// Test Failure (need to inject fail env)
	execCommand = func(command string, args ...string) *exec.Cmd {
		cmd := mockExecCommand(command, args...)
		cmd.Env = append(cmd.Env, "MOCK_GH_FAIL=true")
		return cmd
	}

	_, err = c.CreatePR(".", "Title", "Body", "main")
	if err == nil {
		t.Error("CreatePR expected error, got nil")
	}
}

func TestClient_Bisect(t *testing.T) {
	defer func() {
		execCommand = exec.Command
		execCommandContext = exec.CommandContext
	}()
	execCommand = mockExecCommand
	execCommandContext = mockExecCommandContext

	c := NewClient()
	dir := "."

	// Test BisectStart
	if err := c.BisectStart(dir, "bad", "good"); err != nil {
		t.Errorf("BisectStart failed: %v", err)
	}

	// Test BisectGood
	if err := c.BisectGood(dir, ""); err != nil {
		t.Errorf("BisectGood failed: %v", err)
	}

	// Test BisectBad
	if err := c.BisectBad(dir, ""); err != nil {
		t.Errorf("BisectBad failed: %v", err)
	}

	// Test BisectReset
	if err := c.BisectReset(dir); err != nil {
		t.Errorf("BisectReset failed: %v", err)
	}

	// Test BisectLog
	logs, err := c.BisectLog(dir)
	if err != nil {
		t.Errorf("BisectLog failed: %v", err)
	}
	if len(logs) == 0 {
		t.Error("BisectLog returned empty logs")
	}
}

func TestClient_Diff_Errors(t *testing.T) {
	defer func() {
		execCommand = exec.Command
		execCommandContext = exec.CommandContext
	}()

	c := NewClient()
	dir := "."

	// Test Diff failure
	execCommand = func(command string, args ...string) *exec.Cmd {
		cmd := mockExecCommand(command, args...)
		cmd.Env = append(cmd.Env, "MOCK_GIT_FAIL=true")
		return cmd
	}

	if _, err := c.Diff(dir, "HEAD^", "HEAD"); err == nil {
		t.Error("Diff expected error, got nil")
	}

	// Test DiffStat failure with exit code
	execCommand = func(command string, args ...string) *exec.Cmd {
		cmd := mockExecCommand(command, args...)
		cmd.Env = append(cmd.Env, "MOCK_GIT_EXIT_CODE=1")
		return cmd
	}

	// diff --stat returning exit code 1 is usually an error (unless there are changes? no, 0 is changes/clean, error is >0 usually, but git diff can behave differently.
	// Our Client implementation treats run error as error.
	if _, err := c.DiffStat(dir, "HEAD^", "HEAD"); err == nil {
		t.Error("DiffStat expected error, got nil")
	}
}

func TestClient_Log_Mock(t *testing.T) {
	defer func() {
		execCommand = exec.Command
		execCommandContext = exec.CommandContext
	}()
	execCommand = mockExecCommand

	c := NewClient()
	dir := "."

	logs, err := c.Log(dir, "-n", "1")
	if err != nil {
		t.Errorf("Log failed: %v", err)
	}
	if len(logs) == 0 {
		t.Error("Log returned empty list")
	}

	// Test Log failure
	execCommand = func(command string, args ...string) *exec.Cmd {
		cmd := mockExecCommand(command, args...)
		cmd.Env = append(cmd.Env, "MOCK_GIT_FAIL=true")
		return cmd
	}

	if _, err := c.Log(dir); err == nil {
		t.Error("Log expected error, got nil")
	}
}
