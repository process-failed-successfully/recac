package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOKRCommand(t *testing.T) {
	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "recac-okr-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Override okrFile
	originalOkrFile := okrFile
	okrFile = filepath.Join(tmpDir, ".recac", "okrs.json")
	defer func() { okrFile = originalOkrFile }()

	// Helper to execute command via rootCmd
	execute := func(args ...string) (string, error) {
		buf := new(bytes.Buffer)
		rootCmd.SetOut(buf)
		rootCmd.SetErr(buf)
		rootCmd.SetArgs(args)
		rootCmd.SilenceUsage = true
		err := rootCmd.Execute()
		return buf.String(), err
	}

	t.Run("Init", func(t *testing.T) {
		out, err := execute("okr", "init")
		require.NoError(t, err, "Init failed: %s", out)
		assert.Equal(t, "", out)

		// Verify file exists
		info, err := os.Stat(okrFile)
		require.NoError(t, err)
		fmt.Printf("Created file: %s (%d bytes)\n", okrFile, info.Size())

		// Verify content
		data, err := loadOKRs(okrFile)
		require.NoError(t, err)
		assert.Empty(t, data.Objectives)
	})

	t.Run("Add_Objective", func(t *testing.T) {
		out, err := execute("okr", "add", "Improve Code Quality")
		require.NoError(t, err, "Add Objective failed: %s", out)
		assert.Contains(t, out, "Added Objective O1")

		data, err := loadOKRs(okrFile)
		require.NoError(t, err)
		require.Len(t, data.Objectives, 1)
		assert.Equal(t, "Improve Code Quality", data.Objectives[0].Description)
	})

	t.Run("Add Key Result", func(t *testing.T) {
		// Reset flags
		// Since we run via rootCmd, flags are parsed fresh?
		// No, Cobra flags are persistent state on the command objects.
		// We must reset them manually.
		okrAddKRCmd.Flags().Set("objective", "")
		okrAddKRCmd.Flags().Set("target", "0")
		okrAddKRCmd.Flags().Set("unit", "")
		okrAddKRCmd.Flags().Set("current", "0")

		out, err := execute("okr", "kr", "--objective", "O1", "--target", "80", "--unit", "%", "Test Coverage")
		require.NoError(t, err, "Add KR failed: %s", out)
		assert.Contains(t, out, "Added KR O1-KR1")

		data, err := loadOKRs(okrFile)
		require.NoError(t, err)
		require.Len(t, data.Objectives[0].KeyResults, 1)
		assert.Equal(t, "Test Coverage", data.Objectives[0].KeyResults[0].Description)
		assert.Equal(t, 80.0, data.Objectives[0].KeyResults[0].Target)
	})

	t.Run("Update Key Result", func(t *testing.T) {
		// Reset flags
		okrUpdateCmd.Flags().Set("kr", "")
		okrUpdateCmd.Flags().Set("current", "0")

		out, err := execute("okr", "update", "--kr", "O1-KR1", "--current", "50")
		require.NoError(t, err, "Update KR failed: %s", out)
		assert.Contains(t, out, "Updated O1-KR1 to 50.00")

		data, err := loadOKRs(okrFile)
		require.NoError(t, err)
		assert.Equal(t, 50.0, data.Objectives[0].KeyResults[0].Current)
	})

	t.Run("List OKRs", func(t *testing.T) {
		out, err := execute("okr", "list")
		require.NoError(t, err, "List failed: %s", out)
		assert.Contains(t, out, "OBJECTIVE O1")
		assert.Contains(t, out, "Improve Code Quality")
		assert.Contains(t, out, "KR O1-KR1")
		assert.Contains(t, out, "62.5%") // 50/80 = 0.625
	})

	t.Run("Error Cases", func(t *testing.T) {
		// Reset flags
		okrUpdateCmd.Flags().Set("kr", "")
		okrUpdateCmd.Flags().Set("current", "0")

		out, err := execute("okr", "update", "--kr", "O1-KR99", "--current", "10")
		assert.Error(t, err)
		if err != nil {
			assert.Contains(t, err.Error(), "KR 'O1-KR99' not found")
		}
		fmt.Printf("Error output: %s\n", out)
	})
}
