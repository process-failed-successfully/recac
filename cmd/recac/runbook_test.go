package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestParseRunbook(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "runbook.md")
	content := `
# Header
Some text.

` + "```bash" + `
echo hello
` + "```" + `

More text.
`
	os.WriteFile(file, []byte(content), 0644)

	blocks, err := parseRunbook(file)
	if err != nil {
		t.Fatalf("parseRunbook failed: %v", err)
	}

	if len(blocks) != 3 { // Text, Code, Text
		t.Errorf("Expected 3 blocks, got %d", len(blocks))
	}
	if blocks[1].Type != "code" || blocks[1].Lang != "bash" {
		t.Error("Expected code block")
	}
	if strings.TrimSpace(blocks[1].Content) != "echo hello" {
		t.Error("Expected content 'echo hello'")
	}
}

// TestHelperProcessRunbook is used to mock exec.Command
func TestHelperProcessRunbook(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS_RUNBOOK") != "1" {
		return
	}

	fmt.Fprintf(os.Stderr, "Helper process started. Args: %v\n", os.Args)

	// Verify args or just succeed
	args := os.Args
	found := false
	for i, arg := range args {
		if arg == "-c" {
			// Extract code passed to shell
			code := args[i+1]
			fmt.Fprintf(os.Stderr, "Helper found -c code: %s\n", code)

			// We expect code to contain "env > '...'"
			// We simulate writing env
			if strings.Contains(code, "env >") {
				// Parse path
				parts := strings.Split(code, "'")
				if len(parts) >= 2 {
					path := parts[len(parts)-2]
					fmt.Fprintf(os.Stderr, "Helper extracted path: %s\n", path)

					err := os.WriteFile(path, []byte("NEW_VAR=val"), 0644)
					if err != nil {
						fmt.Fprintf(os.Stderr, "Failed to write file %s: %v\n", path, err)
						os.Exit(1)
					}
					found = true
				} else {
					fmt.Fprintf(os.Stderr, "Helper failed to parse path from code\n")
				}
			} else {
				fmt.Fprintf(os.Stderr, "Helper code does not contain 'env >'\n")
			}
			break
		}
	}
	if !found {
		fmt.Fprintf(os.Stderr, "TestHelperProcessRunbook: -c argument or env pattern not found in args: %v\n", args)
		os.Exit(1)
	}
	os.Exit(0)
}

func TestExecuteBlock(t *testing.T) {
	// Swap exec.Command
	originalExec := runbookExecCommand
	defer func() { runbookExecCommand = originalExec }()
	runbookExecCommand = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestHelperProcessRunbook", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_HELPER_PROCESS_RUNBOOK=1")
		return cmd
	}

	tmpDir := t.TempDir()
	env := map[string]string{
		"OLD": "val",
		"GO_WANT_HELPER_PROCESS_RUNBOOK": "1",
	}

	cmd := &cobra.Command{}
	// We want to see stderr from helper process
	cmd.SetErr(os.Stderr)
	cmd.SetOut(os.Stdout)

	newEnv, err := executeBlock("echo test", env, tmpDir, cmd)
	if err != nil {
		t.Fatalf("executeBlock failed: %v", err)
	}

	if newEnv["NEW_VAR"] != "val" {
		t.Errorf("Expected NEW_VAR=val in output env")
	}
}
