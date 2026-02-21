package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/analysis"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMagicCmd(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Go file with magic literals
	code := `package main
	func main() {
		a := 42
		b := "magic string"
		c := 100
	}
	`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(code), 0644)
	require.NoError(t, err)

	root := &cobra.Command{Use: "recac"}
	root.AddCommand(magicCmd)

	t.Run("Detects Magic Literals", func(t *testing.T) {
		// Reset flags
		magicCmd.Flags().Set("path", tmpDir)
		magicCmd.Flags().Set("min", "1") // Report even single occurrences
		magicCmd.Flags().Set("ignore", "")
		magicCmd.Flags().Set("json", "false")
		magicCmd.Flags().Set("fail", "false")

		output, err := executeCommand(root, "magic", "--path", tmpDir, "--min", "1")
		assert.NoError(t, err)
		assert.Contains(t, output, "VALUE")
		assert.Contains(t, output, "42")
		assert.Contains(t, output, "magic string")
		assert.Contains(t, output, "100")
	})

	t.Run("Ignores Literals", func(t *testing.T) {
		// Strings are kept with quotes in analysis, so we must ignore "\"magic string\""
		output, err := executeCommand(root, "magic", "--path", tmpDir, "--min", "1", "--ignore", "42,\"magic string\"")
		assert.NoError(t, err)
		assert.Contains(t, output, "VALUE")
		assert.NotContains(t, output, "42")
		assert.NotContains(t, output, "\"magic string\"")
		assert.Contains(t, output, "100")
	})

	t.Run("JSON Output", func(t *testing.T) {
		output, err := executeCommand(root, "magic", "--path", tmpDir, "--min", "1", "--json")
		assert.NoError(t, err)

		var findings []analysis.MagicFinding
		err = json.Unmarshal([]byte(output), &findings)
		assert.NoError(t, err)
		assert.Greater(t, len(findings), 0)
	})

	t.Run("Fail Flag", func(t *testing.T) {
		_, err := executeCommand(root, "magic", "--path", tmpDir, "--min", "1", "--fail")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "found")
	})

	t.Run("No Magic Literals", func(t *testing.T) {
		// Create clean file
		cleanDir := t.TempDir()
		cleanCode := `package main
		const MY_CONST = 42
		func main() {
			a := MY_CONST
		}
		`
		err := os.WriteFile(filepath.Join(cleanDir, "clean.go"), []byte(cleanCode), 0644)
		require.NoError(t, err)

		output, err := executeCommand(root, "magic", "--path", cleanDir, "--min", "1")
		assert.NoError(t, err)
		assert.Contains(t, output, "No magic literals found")
	})
}
