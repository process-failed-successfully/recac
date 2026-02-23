package main

import (
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

	// 2. Create Go files with magic literals
	mainGo := `package main
func main() {
	a := 42
	b := 42
	c := 42
	d := "magic"
	e := "magic"
	f := "magic"
}
`
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainGo), 0644); err != nil {
		t.Fatalf("Failed to write main.go: %v", err)
	}

	// 3. Test runMagic
	t.Run("Default", func(t *testing.T) {
		outBuf := new(strings.Builder)
		cmd := &cobra.Command{}
		cmd.SetOut(outBuf)

		magicPath = tmpDir
		magicMinCount = 2
		magicJSON = false
		magicFail = false
		magicIgnore = ""
		defer func() {
			magicPath = "."
			magicMinCount = 0
			magicIgnore = ""
		}()

		err := runMagic(cmd, []string{})
		if err != nil {
			t.Errorf("runMagic failed: %v", err)
		}

		output := outBuf.String()
		if !strings.Contains(output, "42") {
			t.Errorf("Expected output to contain '42', got: %s", output)
		}
		if !strings.Contains(output, "\"magic\"") && !strings.Contains(output, "magic") {
			t.Errorf("Expected output to contain 'magic', got: %s", output)
		}
	})

	t.Run("Ignore", func(t *testing.T) {
		outBuf := new(strings.Builder)
		cmd := &cobra.Command{}
		cmd.SetOut(outBuf)

		magicPath = tmpDir
		magicMinCount = 2
		magicJSON = false
		magicFail = false
		magicIgnore = "42"
		defer func() {
			magicPath = "."
			magicMinCount = 0
			magicIgnore = ""
		}()

		err := runMagic(cmd, []string{})
		if err != nil {
			t.Errorf("runMagic failed: %v", err)
		}

		output := outBuf.String()
		// Tabwriter might output 42 in count column if not careful, but ignore should prevent it from appearing.
		// Wait, if ignored, it shouldn't be in the list.
		if strings.Contains(output, "42\t") { // Check if 42 appears as a value
			t.Errorf("Expected output NOT to contain '42', got: %s", output)
		}
		if !strings.Contains(output, "magic") {
			t.Errorf("Expected output to contain 'magic', got: %s", output)
		}
	})
}
