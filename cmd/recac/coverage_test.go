package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
)

// TestCoverageHelperProcess mocks external commands.
// It is invoked by the test binary when GO_WANT_COVERAGE_HELPER_PROCESS is set.
func TestCoverageHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_COVERAGE_HELPER_PROCESS") != "1" {
		return
	}
	defer os.Exit(0)

	args := os.Args[3:] // Skip binary, -test.run=, --
	if len(args) == 0 {
		return
	}

	cmd := args[0]
	switch cmd {
	case "git":
		// Expect: git diff --unified=0 HEAD
		if len(args) >= 3 && args[1] == "diff" {
			// Return a mock diff
			// main.go: lines 10 and 11 are added.
			fmt.Print(`diff --git a/main.go b/main.go
index 1234567..89abcdef 100644
--- a/main.go
+++ b/main.go
@@ -9,0 +10,2 @@ func foo() {
+	newLine1()
+	newLine2()
`)
		}
	case "go":
		// Expect: go test ./... -coverprofile=coverage.out
		// Just write the file.
		content := `mode: set
github.com/user/repo/main.go:10.1,10.20 1 1
github.com/user/repo/main.go:11.1,11.20 1 0
`
		// Line 10 covered, Line 11 not covered.
		// Note: The profile paths usually include module name.
		// The code logic checks suffix. "main.go" suffix matches "github.com/user/repo/main.go".

		os.WriteFile("coverage.out", []byte(content), 0644)
	}
}

func TestCoverage(t *testing.T) {
	// 1. Setup Mock Exec
	execCommand := func(command string, args ...string) *exec.Cmd {
		cs := []string{"-test.run=TestCoverageHelperProcess", "--", command}
		cs = append(cs, args...)
		cmd := exec.Command(os.Args[0], cs...)
		cmd.Env = append(os.Environ(), "GO_WANT_COVERAGE_HELPER_PROCESS=1")
		return cmd
	}

	// Save old exec and restore after
	oldExec := coverageExec
	coverageExec = execCommand
	defer func() { coverageExec = oldExec }()

	// 2. Setup Temp Dir
	tmpDir := t.TempDir()
	originalWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalWd)

	// Create dummy files
	os.WriteFile("main.go", []byte("package main\n..."), 0644)

	// 3. Run Command - Expect Failure (50% < 80%)
	// Reset flags (cobra flags are global if command is global)
	coverageCmd.Flags().Set("threshold", "80")
	coverageCmd.Flags().Set("branch", "HEAD")

	err := runCoverage(coverageCmd, []string{})

	if err == nil {
		t.Fatal("Expected error due to low coverage, got nil")
	}

	if !strings.Contains(err.Error(), "patch coverage 50.0% is below threshold 80.0%") {
		t.Errorf("Unexpected error message: %v", err)
	}

	// 4. Run with lower threshold - Expect Success
	coverageCmd.Flags().Set("threshold", "40")
	err = runCoverage(coverageCmd, []string{})
	if err != nil {
		t.Errorf("Expected success with low threshold, got: %v", err)
	}
}
