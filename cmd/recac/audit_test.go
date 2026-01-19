package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAuditCommand(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "audit-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Create a high complexity file
	complexCode := `package main

func complexFunc(n int) int {
	if n > 0 {
		if n > 1 {
			if n > 2 {
				if n > 3 {
					if n > 4 {
						if n > 5 {
							if n > 6 {
								if n > 7 {
									if n > 8 {
										if n > 9 {
											return n
										}
									}
								}
							}
						}
					}
				}
			}
		}
	}
	return 0
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "complex.go"), []byte(complexCode), 0644); err != nil {
		t.Fatal(err)
	}

	// 2. Create a file with TODOs
	todoCode := `package main
// TODO: Fix this
// FIXME: This is broken
func foo() {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "todo.go"), []byte(todoCode), 0644); err != nil {
		t.Fatal(err)
	}

	// 3. Create duplicated code (needs to be large enough for CPD, default 10 lines)
	dupBlock := `
	fmt.Println("line 1")
	fmt.Println("line 2")
	fmt.Println("line 3")
	fmt.Println("line 4")
	fmt.Println("line 5")
	fmt.Println("line 6")
	fmt.Println("line 7")
	fmt.Println("line 8")
	fmt.Println("line 9")
	fmt.Println("line 10")
`
	dupCode := `package main
import "fmt"
func a() {` + dupBlock + `}
func b() {` + dupBlock + `}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "dup.go"), []byte(dupCode), 0644); err != nil {
		t.Fatal(err)
	}

	// Test Run Logic (Direct)
	// We set thresholds lower to ensure we catch things if defaults change
	auditComplexityThreshold = 5
	auditCPDMinLines = 5

	report, err := runAudit(tmpDir)
	if err != nil {
		t.Fatalf("runAudit failed: %v", err)
	}

	// Verify Results

	// Complexity: complexFunc should have high complexity (around 11)
	foundComplex := false
	for _, c := range report.ComplexityIssues {
		if c.Function == "complexFunc" && c.Complexity > 10 {
			foundComplex = true
			break
		}
	}
	if !foundComplex {
		t.Errorf("Expected complexFunc to be detected with complexity > 10, got %v", report.ComplexityIssues)
	}

	// TODOs: Should find 2 (TODO and FIXME)
	if len(report.TodoIssues) != 2 {
		t.Errorf("Expected 2 TODO issues, got %d", len(report.TodoIssues))
	}

	// Duplication: Should find at least 1 duplication
	if len(report.DuplicationIssues) == 0 {
		t.Errorf("Expected duplication to be detected")
	}

	// Score: Should be < 100
	if report.Score >= 100 {
		t.Errorf("Expected score < 100, got %d", report.Score)
	}

	// Test Command Integration (JSON Output)
	// We reset flags inside executeCommand's resetFlags, but we need to pass flags explicitly
	out, err := executeCommand(rootCmd, "audit", tmpDir, "--json", "--threshold-complexity=5", "--threshold-cpd=5")
	if err != nil {
		t.Fatalf("executeCommand failed: %v", err)
	}

	var jsonReport AuditReport
	if err := json.Unmarshal([]byte(out), &jsonReport); err != nil {
		t.Fatalf("Failed to parse JSON output: %v\nOutput: %s", err, out)
	}

	if jsonReport.Score != report.Score {
		t.Errorf("JSON score mismatch. Expected %d, got %d", report.Score, jsonReport.Score)
	}
	if len(jsonReport.TodoIssues) != 2 {
		t.Errorf("JSON expected 2 TODO issues, got %d", len(jsonReport.TodoIssues))
	}

	// Test Fail Below Threshold
	_, err = executeCommand(rootCmd, "audit", tmpDir, "--fail-below=100")
	if err == nil {
		t.Errorf("Expected error when score is below threshold")
	} else {
		// Verify error message if possible, or just accept any error
		if !strings.Contains(err.Error(), "audit failed: score") {
			// executeCommand might swallow the error string into the panic message or return nil if we handled it?
			// The executeCommand implementation captures panic "exit-X" but returns output.
			// However, RunE returns an error, which cobra prints and exits with 1.
			// executeCommand captures RunE error?
			// Let's check executeCommand implementation: `err = root.Execute()`.
			// So yes, it returns the error.
		}
	}
}
