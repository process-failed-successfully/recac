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

func createDeadcodeTestFiles(t *testing.T, dir string) {
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
	if err := os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}
}

func TestDeadcodeAnalysis(t *testing.T) {
	tmpDir := t.TempDir()
	createDeadcodeTestFiles(t, tmpDir)

	// Run analysis
	findings, err := analyzeDeadcode(tmpDir)
	if err != nil {
		t.Fatalf("Analysis failed: %v", err)
	}

	// Assertions
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
	b, err := json.Marshal(findings)
	if err != nil {
		t.Errorf("Failed to marshal findings: %v", err)
	}
	if len(b) == 0 {
		t.Error("JSON output is empty")
	}
}

func TestRunDeadcode(t *testing.T) {
	tmpDir := t.TempDir()
	createDeadcodeTestFiles(t, tmpDir)

	// Save/Restore global flags
	origJSON := deadcodeJSON
	origFail := deadcodeFail
	origStrict := deadcodeStrict
	defer func() {
		deadcodeJSON = origJSON
		deadcodeFail = origFail
		deadcodeStrict = origStrict
	}()

	t.Run("Text Output", func(t *testing.T) {
		deadcodeJSON = false
		deadcodeFail = false
		deadcodeStrict = false

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		if err := runDeadcode(cmd, []string{tmpDir}); err != nil {
			t.Fatalf("runDeadcode failed: %v", err)
		}

		output := buf.String()
		if !strings.Contains(output, "UnusedFunc") {
			t.Error("Output should contain UnusedFunc")
		}
		if !strings.Contains(output, "UnusedType") {
			t.Error("Output should contain UnusedType")
		}
	})

	t.Run("JSON Output", func(t *testing.T) {
		deadcodeJSON = true
		deadcodeFail = false
		deadcodeStrict = false

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		if err := runDeadcode(cmd, []string{tmpDir}); err != nil {
			t.Fatalf("runDeadcode failed: %v", err)
		}

		output := buf.String()
		var findings []DeadcodeFinding
		if err := json.Unmarshal(buf.Bytes(), &findings); err != nil {
			t.Fatalf("Failed to unmarshal JSON output: %v\nOutput: %s", err, output)
		}

		if len(findings) == 0 {
			t.Error("Expected findings in JSON output")
		}
	})

	t.Run("Fail Flag", func(t *testing.T) {
		deadcodeJSON = false
		deadcodeFail = true // Should return error
		deadcodeStrict = false

		cmd := &cobra.Command{}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := runDeadcode(cmd, []string{tmpDir})
		if err == nil {
			t.Error("Expected error when fail flag is set and findings exist")
		}
	})
}
