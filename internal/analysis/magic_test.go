package analysis

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractMagicLiterals(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "magic_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Define test files
	files := map[string]string{
		"main.go": `package main

import "fmt"

const ConstVal = 100
const (
	ConstGroup = "skip_me"
)

type MyStruct struct {
	Field int ` + "`json:\"skip_tag\"`" + `
}

func main() {
	var a = 42
	b := 42
	fmt.Println("magic_string")
	if a == 0 { // 0 is default ignored
		fmt.Println("ignored")
	}
	doSomething(3.14)
}

func doSomething(f float64) {
	_ = "magic_string"
}
`,
	}

	for name, content := range files {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Run extraction
	findings, err := ExtractMagicLiterals(tmpDir, nil)
	if err != nil {
		t.Fatalf("ExtractMagicLiterals failed: %v", err)
	}

	// Analyze results
	results := make(map[string]MagicFinding)
	for _, f := range findings {
		results[f.Value] = f
	}

	// Assertions
	checkFinding(t, results, "42", 2, "INT")
	checkFinding(t, results, "\"magic_string\"", 2, "STRING")
	checkFinding(t, results, "3.14", 1, "FLOAT")

	checkMissing(t, results, "100")       // Const
	checkMissing(t, results, "\"skip_me\"") // Const
	checkMissing(t, results, "\"skip_tag\"") // Tag
	checkMissing(t, results, "\"fmt\"")     // Import
	checkMissing(t, results, "0")           // Default ignore
}

func checkFinding(t *testing.T, results map[string]MagicFinding, val string, count int, expectedType string) {
	f, ok := results[val]
	if !ok {
		t.Errorf("Expected to find magic literal %s, but didn't", val)
		return
	}
	if f.Occurrences != count {
		t.Errorf("Expected %d occurrences of %s, got %d", count, val, f.Occurrences)
	}
	if f.Type != expectedType {
		t.Errorf("Expected type %s for %s, got %s", expectedType, val, f.Type)
	}
}

func checkMissing(t *testing.T, results map[string]MagicFinding, val string) {
	if _, ok := results[val]; ok {
		t.Errorf("Did not expect to find literal %s, but did", val)
	}
}
