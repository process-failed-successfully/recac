package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
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

	// 2. Create files
	mainGo := `package main

import "fmt"

func main() {
}

func UnusedFunc() {
	fmt.Println("Unused")
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// 3. Setup Command
	cmd := &cobra.Command{
		Use: "deadcode",
		RunE: runDeadcode,
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// 4. Run Command
	// We pass the directory as argument
	err = runDeadcode(cmd, []string{tmpDir})
	assert.NoError(t, err)

	// 5. Verify Output
	output := buf.String()
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "IDENTIFIER")
	assert.Contains(t, output, "UnusedFunc")

	// Test JSON output
	deadcodeJSON = true
	defer func() { deadcodeJSON = false }()
	buf.Reset()

	err = runDeadcode(cmd, []string{tmpDir})
	assert.NoError(t, err)

	output = buf.String()
	assert.Contains(t, output, "[") // JSON array start
	assert.Contains(t, output, "\"identifier\": \"UnusedFunc\"")
}

func TestDeadcodeValueSpecAndField(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-deadcode-run-test2")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainGo := "package main\n\nimport \"fmt\"\n\ntype UsedType struct {\n\tField int `json:\"field\"`\n}\n\nvar MyVar UsedType = UsedType{Field: 1}\n\nfunc main() {\n\tfmt.Println(MyVar)\n}\n"

    if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

    testGo := "package main\n\nimport \"fmt\"\n\nfunc TestA() {\nfmt.Println(\"testing\")\n}"

    if err := os.WriteFile(filepath.Join(tmpDir, "main_test.go"), []byte(testGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

    // Create an ignored dir
    ignoredDir := filepath.Join(tmpDir, "vendor")
    if err := os.MkdirAll(ignoredDir, 0755); err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}

    vendorGo := "package main\n\nimport \"fmt\"\n\nfunc UnusedFunc() {\nfmt.Println(\"testing\")\n}"
    if err := os.WriteFile(filepath.Join(ignoredDir, "vendor.go"), []byte(vendorGo), 0644); err != nil {
		t.Fatalf("Failed to write vendor.go: %v", err)
	}

	findings, err := analyzeDeadcode(tmpDir)
	assert.NoError(t, err)

    // We expect nothing to be unused
    assert.Empty(t, findings)
}

func TestRunDeadcodeFail(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-deadcode-run-fail-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainGo := "package main\n\nimport \"fmt\"\n\nfunc UnusedFunc() {\n\tfmt.Println(\"Unused\")\n}\n"
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	cmd := &cobra.Command{
		Use: "deadcode",
		RunE: runDeadcode,
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	deadcodeFail = true
	defer func() { deadcodeFail = false }()

	err = runDeadcode(cmd, []string{tmpDir})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "found 1 unused identifiers")
}

func TestDeadcodeTypeSpec(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-deadcode-run-test-typespec")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainGo := "package main\n\nimport \"fmt\"\n\ntype MyType struct{}\n\ntype AliasType MyType\n\nfunc main() {\n\tfmt.Println(AliasType{})\n}\n"

    if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	findings, err := analyzeDeadcode(tmpDir)
	assert.NoError(t, err)

    // We expect nothing to be unused
    assert.Empty(t, findings)
}


func TestDeadcodeTypeSpecUnused(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-deadcode-run-test-typespec2")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainGo := "package main\n\ntype MyType struct{}\n\nvar unusedVar MyType\n"

    if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	_, err = analyzeDeadcode(tmpDir)
	assert.NoError(t, err)
}


func TestDeadcodeFieldWithoutTag(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-deadcode-run-test-notag")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainGo := "package main\n\nimport \"fmt\"\n\ntype MyType struct{\n\tField int\n}\n\nfunc main() {\n\tvar t MyType\n\tfmt.Println(t.Field)\n}\n"

    if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	_, err = analyzeDeadcode(tmpDir)
	assert.NoError(t, err)
}

func TestDeadcodeValueSpecWithoutType(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "recac-deadcode-run-test-notype")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	mainGo := "package main\n\nimport \"fmt\"\n\ntype UsedType struct {}\n\nvar MyVar = UsedType{}\n\nfunc main() {\n\tfmt.Println(MyVar)\n}\n"

    if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	_, err = analyzeDeadcode(tmpDir)
	assert.NoError(t, err)
}
