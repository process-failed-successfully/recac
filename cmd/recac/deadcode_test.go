package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestDeadcodeAnalysis(t *testing.T) {
	// 1. Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "recac-deadcode-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Create files
	mainGo := `package main

import "fmt"

func main() {
	UsedFunc()
	fmt.Println("Hello")
}

func UsedFunc() {
	fmt.Println("Used")
}

func UnusedFunc() {
	fmt.Println("Unused")
}

type UsedType struct {
	Field int
}

type UnusedType struct {
	Field int
}

func (u *UsedType) UsedMethod() {
}

func (u *UsedType) UnusedMethod() {
}

func (u *UnusedType) UnusedMethodOnUnusedType() {
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// 3. Run analysis
	findings, err := analyzeDeadcode(tmpDir)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// 4. Assertions
	foundUnusedFunc := false
	foundUnusedType := false
	foundUnusedMethod := false
	foundUsedFunc := false

	for _, f := range findings {
		// Log for debugging
		t.Logf("Finding: %s (%s)", f.Identifier, f.Type)

		if f.Identifier == "UnusedFunc" {
			foundUnusedFunc = true
		}
		if f.Identifier == "UnusedType" {
			foundUnusedType = true
		}
		if strings.Contains(f.Identifier, "UnusedMethod") {
			foundUnusedMethod = true
		}
		if f.Identifier == "UsedFunc" {
			foundUsedFunc = true
		}
	}

	if !foundUnusedFunc {
		t.Error("Expected to find UnusedFunc")
	}
	if !foundUnusedType {
		t.Error("Expected to find UnusedType")
	}
	if !foundUnusedMethod {
		t.Error("Expected to find UnusedMethod")
	}
	if foundUsedFunc {
		t.Error("Did not expect to find UsedFunc")
	}

	// Test JSON output logic (integration style)
	// We can't easily mock stdout here without refactoring `runDeadcode` to take an io.Writer.
	// But we can check if analyzeDeadcode returns valid structs.
	b, err := json.Marshal(findings)
	if err != nil {
		t.Errorf("Failed to marshal findings: %v", err)
	}
	if len(b) == 0 {
		t.Error("JSON output is empty")
	}
}

func TestRunDeadcode(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	mainGo := `package main
func UnusedFunc() {}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatal(err)
	}

	var outBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	// Test default output (table)
	err := runDeadcode(cmd, []string{tmpDir})
	if err != nil {
		t.Errorf("runDeadcode failed: %v", err)
	}
	output := outBuf.String()
	if !strings.Contains(output, "UnusedFunc") {
		t.Errorf("Output should contain UnusedFunc, got: %s", output)
	}

	// Test JSON output
	outBuf.Reset()
	deadcodeJSON = true

	err = runDeadcode(cmd, []string{tmpDir})
	if err != nil {
		t.Errorf("runDeadcode JSON failed: %v", err)
	}
	output = outBuf.String()
	if !strings.Contains(output, "\"identifier\": \"UnusedFunc\"") {
		t.Errorf("JSON output should contain UnusedFunc, got: %s", output)
	}

	// Reset JSON flag
	deadcodeJSON = false

	// Test Fail Flag
	deadcodeFail = true
	defer func() { deadcodeFail = false }()
	err = runDeadcode(cmd, []string{tmpDir})
	if err == nil {
		t.Error("Expected error when deadcodeFail is true and findings exist")
	}

	// Test No Findings
	os.Remove(filepath.Join(tmpDir, "main.go"))
	// Create clean file
	cleanGo := `package main
func main() {}
`
	os.WriteFile(filepath.Join(tmpDir, "clean.go"), []byte(cleanGo), 0644)

	outBuf.Reset()
	deadcodeFail = true // Should not fail if no findings
	deadcodeJSON = false

	err = runDeadcode(cmd, []string{tmpDir})
	if err != nil {
		t.Errorf("runDeadcode failed on clean project: %v", err)
	}
	output = outBuf.String()
	if !strings.Contains(output, "No dead code found") {
		t.Errorf("Expected 'No dead code found', got: %s", output)
	}
}
