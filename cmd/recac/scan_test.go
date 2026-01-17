package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/db"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanCmd(t *testing.T) {
	// Setup temp directory
	tempDir, err := os.MkdirTemp("", "recac-scan-test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a file with TODOs
	file1 := filepath.Join(tempDir, "main.go")
	content1 := `package main
	// TODO: Fix this later
	func main() {
		// FIXME(jules): This is broken
	}
	`
	err = os.WriteFile(file1, []byte(content1), 0644)
	require.NoError(t, err)

	// Create a file in an ignored directory
	gitDir := filepath.Join(tempDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	require.NoError(t, err)
	file2 := filepath.Join(gitDir, "ignore_me.go")
	content2 := `// TODO: Ignore me`
	err = os.WriteFile(file2, []byte(content2), 0644)
	require.NoError(t, err)

	// Switch to temp dir so the command runs there
	cwd, _ := os.Getwd()
	defer os.Chdir(cwd)
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	t.Run("Scan Text Output", func(t *testing.T) {
		cmd := NewScanCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "TODO")
		assert.Contains(t, output, "Fix this later")
		assert.Contains(t, output, "FIXME")
		assert.Contains(t, output, "broken")
		assert.NotContains(t, output, "Ignore me")
	})

	t.Run("Scan JSON Output", func(t *testing.T) {
		cmd := NewScanCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.Flags().Set("json", "true")
		require.NoError(t, err)

		err = cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		var results []ScanResult
		err = json.Unmarshal([]byte(output), &results)
		require.NoError(t, err)

		assert.Len(t, results, 2)
		// Verify contents
		foundTodo := false
		foundFixme := false
		for _, r := range results {
			if r.Type == "TODO" {
				foundTodo = true
			}
			if r.Type == "FIXME" {
				foundFixme = true
			}
		}
		assert.True(t, foundTodo)
		assert.True(t, foundFixme)
	})

	t.Run("Export Plan", func(t *testing.T) {
		cmd := NewScanCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		err := cmd.Flags().Set("export-plan", "plan.json")
		require.NoError(t, err)

		err = cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		// Verify plan.json exists
		planBytes, err := os.ReadFile("plan.json")
		require.NoError(t, err)

		var plan db.FeatureList
		err = json.Unmarshal(planBytes, &plan)
		require.NoError(t, err)

		assert.NotEmpty(t, plan.Features)
		assert.Contains(t, plan.Features[0].Description, "TODO")
	})

	t.Run("False Positives", func(t *testing.T) {
		cmd := NewScanCmd()
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)

		// Create file with false positives
		fpFile := filepath.Join(tempDir, "false_positives.go")
		fpContent := `package main
		func debug() {
			log.Debug("This is not a BUG")
			var shacks = "Not HACKs"
		}
		`
		err := os.WriteFile(fpFile, []byte(fpContent), 0644)
		require.NoError(t, err)

		err = cmd.RunE(cmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		// Should NOT contain lines that are not markers
		// We check for the content of the lines that shouldn't be matched
		assert.NotContains(t, output, "This is not a BUG")
		assert.NotContains(t, output, "Not HACKs")

		// Ensure it still finds the real ones from previous setup
		assert.Contains(t, output, "Fix this later")
	})
}
