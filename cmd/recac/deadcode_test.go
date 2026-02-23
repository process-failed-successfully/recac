package main

import (
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
	// 1. Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "recac-deadcode-run-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Create files with unused code
	mainGo := `package main
import "fmt"
func main() { fmt.Println("Hello") }
func UnusedRunFunc() { fmt.Println("Unused") }
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// 3. Test default output (text)
	t.Run("Text Output", func(t *testing.T) {
		// Mock stdout
		outBuf := new(strings.Builder)
		cmd := &cobra.Command{}
		cmd.SetOut(outBuf)

		// Reset flags (globals)
		deadcodeJSON = false
		deadcodeFail = false
		deadcodeStrict = false

		err := runDeadcode(cmd, []string{tmpDir})
		if err != nil {
			t.Errorf("runDeadcode failed: %v", err)
		}

		output := outBuf.String()
		if !strings.Contains(output, "UnusedRunFunc") {
			t.Errorf("Expected output to contain 'UnusedRunFunc', got: %s", output)
		}
		if !strings.Contains(output, "TYPE") { // Header check
			t.Errorf("Expected output to contain header 'TYPE', got: %s", output)
		}
	})

	// 4. Test JSON output
	t.Run("JSON Output", func(t *testing.T) {
		outBuf := new(strings.Builder)
		cmd := &cobra.Command{}
		cmd.SetOut(outBuf)

		deadcodeJSON = true
		deadcodeFail = false
		defer func() { deadcodeJSON = false }()

		err := runDeadcode(cmd, []string{tmpDir})
		if err != nil {
			t.Errorf("runDeadcode failed: %v", err)
		}

		output := outBuf.String()
		if !strings.Contains(output, "\"identifier\": \"UnusedRunFunc\"") {
			t.Errorf("Expected JSON output to contain identifier 'UnusedRunFunc', got: %s", output)
		}
	})

	// 5. Test Fail flag
	t.Run("Fail Flag", func(t *testing.T) {
		outBuf := new(strings.Builder)
		cmd := &cobra.Command{}
		cmd.SetOut(outBuf)

		deadcodeJSON = false
		deadcodeFail = true
		defer func() { deadcodeFail = false }()

		err := runDeadcode(cmd, []string{tmpDir})
		if err == nil {
			t.Error("Expected error when deadcodeFail is true and issues found")
		} else if !strings.Contains(err.Error(), "found 1 unused identifiers") {
			t.Errorf("Unexpected error message: %v", err)
		}
	})

	// 6. Test Clean (No deadcode)
	t.Run("Clean Code", func(t *testing.T) {
		cleanDir, _ := os.MkdirTemp("", "recac-deadcode-clean")
		defer os.RemoveAll(cleanDir)
		os.WriteFile(filepath.Join(cleanDir, "clean.go"), []byte("package main\nfunc main(){}"), 0644)

		outBuf := new(strings.Builder)
		cmd := &cobra.Command{}
		cmd.SetOut(outBuf)
		deadcodeFail = true // Should not fail if clean
		defer func() { deadcodeFail = false }()

		err := runDeadcode(cmd, []string{cleanDir})
		if err != nil {
			t.Errorf("runDeadcode failed on clean code: %v", err)
		}
		if !strings.Contains(outBuf.String(), "No dead code found") {
			t.Errorf("Expected success message, got: %s", outBuf.String())
		}
	})
}
