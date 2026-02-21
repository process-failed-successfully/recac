package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnalyzeDeadcode(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. main.go with unused exported function and used function
	mainContent := `package main

// UnusedFunc should be reported
func UnusedFunc() {}

// UsedFunc is used in main, should not be reported
func UsedFunc() {}

// UnusedType should be reported
type UnusedType struct{}

func main() {
	UsedFunc()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// 2. lib/lib.go with exported function (library code, not necessarily used internally)
	libDir := filepath.Join(tmpDir, "lib")
	err = os.Mkdir(libDir, 0755)
	require.NoError(t, err)

	libContent := `package lib

// ExportedLibFunc should be ignored unless strict
func ExportedLibFunc() {}
`
	err = os.WriteFile(filepath.Join(libDir, "lib.go"), []byte(libContent), 0644)
	require.NoError(t, err)

	t.Run("Default Mode (Not Strict)", func(t *testing.T) {
		// Ensure global flag is false
		originalStrict := deadcodeStrict
		deadcodeStrict = false
		defer func() { deadcodeStrict = originalStrict }()

		findings, err := analyzeDeadcode(tmpDir)
		require.NoError(t, err)

		foundUnusedFunc := false
		foundUnusedType := false
		foundLibFunc := false
		foundUsedFunc := false

		for _, f := range findings {
			if f.Identifier == "UnusedFunc" {
				foundUnusedFunc = true
			}
			if f.Identifier == "UnusedType" {
				foundUnusedType = true
			}
			if f.Identifier == "ExportedLibFunc" {
				foundLibFunc = true
			}
			if f.Identifier == "UsedFunc" {
				foundUsedFunc = true
			}
		}

		assert.True(t, foundUnusedFunc, "Should report UnusedFunc in main package")
		assert.True(t, foundUnusedType, "Should report UnusedType in main package")
		assert.False(t, foundLibFunc, "Should NOT report ExportedLibFunc in lib package by default")
		assert.False(t, foundUsedFunc, "Should NOT report UsedFunc as it is used")
	})

	t.Run("Strict Mode", func(t *testing.T) {
		// Set global flag true
		originalStrict := deadcodeStrict
		deadcodeStrict = true
		defer func() { deadcodeStrict = originalStrict }()

		findings, err := analyzeDeadcode(tmpDir)
		require.NoError(t, err)

		foundLibFunc := false

		for _, f := range findings {
			if f.Identifier == "ExportedLibFunc" {
				foundLibFunc = true
			}
		}

		assert.True(t, foundLibFunc, "Should report ExportedLibFunc in strict mode")
	})
}
