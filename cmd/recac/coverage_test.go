package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// Mocking the exec.CommandContext for tests
func TestRunCoverage(t *testing.T) {
	// Override the execCommand to use our mock
	coverageExec = func(name string, arg ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCoverageHelperProcess", "--", name}
		cs = append(cs, arg...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = []string{"GO_WANT_HELPER_PROCESS=1"}
		return cmd
	}

	// Create a dummy coverage profile
	profileContent := `mode: set
github.com/recac/pkg/main.go:10.10,12.2 2 1
github.com/recac/pkg/main.go:14.1,16.5 3 0
github.com/recac/pkg/utils.go:5.1,10.1 5 1
`
	if err := os.WriteFile("coverage.out", []byte(profileContent), 0644); err != nil {
		t.Fatal(err)
	}
	defer os.Remove("coverage.out")

	// Set flags
	covProfile = "coverage.out"
	covThreshold = 0
	covPatchThreshold = 0
	covRunCmd = "echo running tests" // Mock command

	cmd := coverageCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Run
	if err := runCoverage(cmd, []string{}); err != nil {
		t.Errorf("runCoverage failed: %v", err)
	}

	output := buf.String()
	// Check output
	if !containsStr(output, "Total Coverage:") {
		t.Error("Output missing total coverage")
	}
	if !containsStr(output, "Patch Coverage:") {
		t.Error("Output missing patch coverage")
	}
}

func containsStr(s, substr string) bool {
	return bytes.Contains([]byte(s), []byte(substr))
}

// Reuse TestHelperProcess from commands_test.go if possible, or use a new name
// Since commands_test.go has TestHelperProcess, we can rely on it if it handles our cases.
// But TestHelperProcess in commands_test.go might not handle our specific args.
// Let's define TestCoverageHelperProcess.

func TestCoverageHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

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

	cmd := args[0]

	// Mock behavior based on command
	switch cmd {
	case "git":
		if len(args) > 1 && args[1] == "diff" {
			// Return a mock diff
			// Modify main.go lines 14-16 (which are uncovered in our profile)
			fmt.Print(`diff --git a/github.com/recac/pkg/main.go b/github.com/recac/pkg/main.go
index abc..def 100644
--- a/github.com/recac/pkg/main.go
+++ b/github.com/recac/pkg/main.go
@@ -13,0 +14,3 @@
+	if err != nil {
+		return err
+	}
`)
		}
	case "sh":
		// Mock test run
		fmt.Println("PASS")
	case "go":
		if len(args) > 1 && args[1] == "test" {
			// Mock test run
			// We already created the profile file in the test setup
		}
	}
}

// Override coverageExec in this file to use TestCoverageHelperProcess
// We need to update the TestRunCoverage to point to TestCoverageHelperProcess
func init() {
	// We can't easily change the name of the helper process called by coverageExec in the test function
	// because it constructs the args.
	// We will update TestRunCoverage to use TestCoverageHelperProcess.
}
