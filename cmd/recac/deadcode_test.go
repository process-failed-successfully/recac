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
	tmpDir := t.TempDir()

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
	// We check struct compatibility here
	b, err := json.Marshal(findings)
	if err != nil {
		t.Errorf("Failed to marshal findings: %v", err)
	}
	if len(b) == 0 {
		t.Error("JSON output is empty")
	}
}

func TestRunDeadcode(t *testing.T) {
	// Create temp dir with deadcode
	tmpDir := t.TempDir()
	mainGo := `package main
func Unused() {}
func main() {}`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("failed to write main.go: %v", err)
	}

	// Case 1: Text Output
	cmd := &cobra.Command{Use: "deadcode", RunE: runDeadcode}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// deadcodeJSON is global
	oldJSON := deadcodeJSON
	defer func() { deadcodeJSON = oldJSON }()
	deadcodeJSON = false

	if err := runDeadcode(cmd, []string{tmpDir}); err != nil {
		t.Fatalf("runDeadcode failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Unused") {
		t.Error("expected 'Unused' in text output")
	}
	if !strings.Contains(output, "TYPE") {
		t.Error("expected table header 'TYPE'")
	}

	// Case 2: JSON Output
	buf.Reset()
	deadcodeJSON = true
	if err := runDeadcode(cmd, []string{tmpDir}); err != nil {
		t.Fatalf("runDeadcode JSON failed: %v", err)
	}

	var findings []DeadcodeFinding
	if err := json.Unmarshal(buf.Bytes(), &findings); err != nil {
		t.Fatalf("failed to unmarshal JSON output: %v", err)
	}
	if len(findings) == 0 {
		t.Error("expected findings in JSON output")
	}
	if findings[0].Identifier != "Unused" {
		t.Errorf("expected identifier Unused, got %s", findings[0].Identifier)
	}
}
