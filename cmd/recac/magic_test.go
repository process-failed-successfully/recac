package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"recac/internal/analysis"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMagicCmd(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir := t.TempDir()

	// 2. Create Files with Magic Literals
	// We put all files in one dir, but tests can specify different paths if needed
	files := map[string]string{
		"main.go": `package main
import "fmt"
func main() {
	i := 42
	s := "magic"
	fmt.Println(i, s)
}`,
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Helper to run magic command
	// We need to carefully manage global variables because runMagic uses them directly
	runMagicTest := func(t *testing.T, args ...string) (string, error) {
		// Reset globals to defaults or test defaults
		magicPath = tmpDir
		magicMinCount = 1 // Set to 1 to catch single occurrences for testing
		magicIgnore = ""
		magicJSON = false
		magicFail = false

		// Parse args manually to simulate flag setting
		for _, arg := range args {
			if arg == "--json" {
				magicJSON = true
			} else if arg == "--fail" {
				magicFail = true
			} else if strings.HasPrefix(arg, "--ignore=") {
				magicIgnore = strings.TrimPrefix(arg, "--ignore=")
			} else if strings.HasPrefix(arg, "--path=") {
				magicPath = strings.TrimPrefix(arg, "--path=")
			}
		}

		cmd := &cobra.Command{Use: "magic", RunE: runMagic}
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		// We ignore cmd args since we set globals directly
		err := runMagic(cmd, []string{})
		return buf.String(), err
	}

	// Test 1: Basic Detection
	t.Run("Basic Detection", func(t *testing.T) {
		output, err := runMagicTest(t)
		require.NoError(t, err)
		assert.Contains(t, output, "42")
		assert.Contains(t, output, "magic")
	})

	// Test 2: JSON Output
	t.Run("JSON Output", func(t *testing.T) {
		output, err := runMagicTest(t, "--json")
		require.NoError(t, err)

		var findings []analysis.MagicFinding
		err = json.Unmarshal([]byte(output), &findings)
		require.NoError(t, err)

		found42 := false
		for _, f := range findings {
			if f.Value == "42" {
				found42 = true
				break
			}
		}
		assert.True(t, found42)
	})

	// Test 3: Ignore
	t.Run("Ignore", func(t *testing.T) {
		output, err := runMagicTest(t, "--ignore=42")
		require.NoError(t, err)
		// Since we ignored 42, it should NOT be in output, except maybe in paths, so split and check carefully
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.HasPrefix(line, "42") || strings.Contains(line, " 42 ") {
				t.Errorf("Should ignore 42, but found in line: %s", line)
			}
		}
		// "magic" string should still be there
		assert.Contains(t, output, "magic")
	})

	// Test 4: Fail
	t.Run("Fail", func(t *testing.T) {
		_, err := runMagicTest(t, "--fail")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "found")
	})

	// Test 5: Clean
	t.Run("Clean", func(t *testing.T) {
		cleanDir := t.TempDir()
		// We pass path via args helper logic
		output, err := runMagicTest(t, fmt.Sprintf("--path=%s", cleanDir))
		require.NoError(t, err)
		assert.Contains(t, output, "No magic literals found")
	})
}
