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
	tmpDir := t.TempDir()

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
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	libGo := `package lib

func UnusedLibFunc() {
}
`
	if err := os.Mkdir(filepath.Join(tmpDir, "lib"), 0755); err != nil {
		t.Fatalf("Failed to create lib dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "lib", "lib.go"), []byte(libGo), 0644); err != nil {
		t.Fatalf("Failed to write lib.go: %v", err)
	}

	tests := []struct {
		name        string
		strict      bool
		json        bool
		fail        bool
		wantErr     bool
		wantOut     []string
		wantMissing []string
	}{
		{
			name:    "Default",
			wantOut: []string{"TYPE", "IDENTIFIER", "UnusedFunc"},
		},
		{
			name:    "JSON",
			json:    true,
			wantOut: []string{`"identifier": "UnusedFunc"`},
		},
		{
			name:    "Fail Flag",
			fail:    true,
			wantErr: true, // Should fail because UnusedFunc exists
			wantOut: []string{"TYPE", "IDENTIFIER"},
		},
		{
			name:    "Strict Mode",
			strict:  true,
			wantOut: []string{"UnusedLibFunc"},
		},
		{
			name:    "Non-Strict (Lib Ignored)",
			strict:  false,
			// UnusedLibFunc should NOT be present
			// But UnusedFunc (main) SHOULD be present
			wantOut:     []string{"UnusedFunc"},
			wantMissing: []string{"UnusedLibFunc"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset globals
			deadcodeJSON = tt.json
			deadcodeFail = tt.fail
			deadcodeStrict = tt.strict

			cmd := &cobra.Command{}
			buf := new(bytes.Buffer)
			cmd.SetOut(buf)

			// We need to pass the path as argument
			args := []string{tmpDir}

			err := runDeadcode(cmd, args)
			if (err != nil) != tt.wantErr {
				t.Errorf("runDeadcode() error = %v, wantErr %v", err, tt.wantErr)
			}

			output := buf.String()
			for _, want := range tt.wantOut {
				if !strings.Contains(output, want) {
					t.Errorf("runDeadcode() output missing %q, got:\n%s", want, output)
				}
			}
			for _, missing := range tt.wantMissing {
				if strings.Contains(output, missing) {
					t.Errorf("runDeadcode() output unexpected %q, got:\n%s", missing, output)
				}
			}
		})
	}
}
