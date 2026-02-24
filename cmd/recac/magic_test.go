package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunMagic(t *testing.T) {
	// 1. Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "recac-magic-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 2. Create files with magic literals
	mainGo := `package main

import "fmt"

func main() {
	// 42 appears twice (minimum is 2 by default)
	fmt.Println(42)
	fmt.Println(42)

	// "magic" appears twice
	fmt.Println("magic")
	fmt.Println("magic")

	// 100 appears only once
	fmt.Println(100)
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// 3. Mock Command
	cmd := &cobra.Command{}
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Save/Restore flags
	oldPath := magicPath
	oldMin := magicMinCount
	oldIgnore := magicIgnore
	oldJSON := magicJSON
	oldFail := magicFail

	defer func() {
		magicPath = oldPath
		magicMinCount = oldMin
		magicIgnore = oldIgnore
		magicJSON = oldJSON
		magicFail = oldFail
	}()

	// Configure flags
	magicPath = tmpDir
	magicMinCount = 2
	magicIgnore = ""
	magicJSON = false
	magicFail = false

	// Test 1: Normal output
	if err := runMagic(cmd, []string{}); err != nil {
		t.Fatalf("runMagic failed: %v", err)
	}

	output := buf.String()
	t.Logf("Output: %s", output)

	if !strings.Contains(output, "42") {
		t.Error("Output should report 42")
	}
	if !strings.Contains(output, "\"magic\"") {
		t.Error("Output should report \"magic\"")
	}
	if strings.Contains(output, "100") {
		t.Error("Output should NOT report 100 (count < min)")
	}

	// Test 2: Ignore
	buf.Reset()
	magicIgnore = "42"
	if err := runMagic(cmd, []string{}); err != nil {
		t.Fatalf("runMagic (ignore) failed: %v", err)
	}
	output = buf.String()
	if strings.Contains(output, "42") {
		t.Error("Output should NOT report 42 (ignored)")
	}
	if !strings.Contains(output, "\"magic\"") {
		t.Error("Output should still report \"magic\"")
	}

	// Test 3: JSON
	buf.Reset()
	magicJSON = true
	magicIgnore = ""
	if err := runMagic(cmd, []string{}); err != nil {
		t.Fatalf("runMagic (json) failed: %v", err)
	}
	output = buf.String()
	if !strings.Contains(output, "\"value\": \"42\"") {
		t.Error("JSON output should contain 42")
	}

	// Test 4: Fail
	buf.Reset()
	magicJSON = false
	magicFail = true
	if err := runMagic(cmd, []string{}); err == nil {
		t.Error("runMagic should fail when magic literals found and --fail is set")
	}
}
