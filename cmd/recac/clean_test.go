package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCleanCmd(t *testing.T) {
	// Create a temporary directory
	tempDir := t.TempDir()

	// Switch to the temporary directory
	originalWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current working directory: %v", err)
	}
	defer os.Chdir(originalWd)
	if err := os.Chdir(tempDir); err != nil {
		t.Fatalf("Failed to change to temp dir: %v", err)
	}

	// Create dummy files to be cleaned via temp_files.txt
	file1 := "file1.txt"
	file2 := "subdir/file2.txt"

	// Ephemeral files that should be cleaned by extension/name
	ephemeral1 := "coverage.out"
	ephemeral2 := "app_spec.txt"
	ephemeral3 := "feature_list.json"
	ephemeral4 := ".recac-session123.log"

	// Valid files that should NOT be cleaned
	keep1 := "main.go"
	keep2 := "subdir/useful.txt"
	keep3 := "test.diff" // should no longer be cleaned automatically
	keep4 := "patch.patch"

	allFiles := []string{file1, file2, ephemeral1, ephemeral2, ephemeral3, ephemeral4, keep1, keep2, keep3, keep4}

	for _, f := range allFiles {
		if err := os.MkdirAll(filepath.Dir(f), 0755); err != nil {
			t.Fatalf("Failed to create dir for %s: %v", f, err)
		}
		if err := os.WriteFile(f, []byte("content"), 0644); err != nil {
			t.Fatalf("Failed to write %s: %v", f, err)
		}
	}

	// Create temp_files.txt listing specific files
	tempFilesList := "temp_files.txt"
	content := file1 + "\n" + file2 + "\n"
	if err := os.WriteFile(tempFilesList, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp_files.txt: %v", err)
	}

	// Test dry run mode first
	cleanDryRun = true
	var outBuf bytes.Buffer
	cleanCmd.SetOut(&outBuf)
	err = cleanCmd.RunE(cleanCmd, []string{})
	if err != nil {
		t.Fatalf("cleanCmd dry-run failed: %v", err)
	}

	// Verify files are NOT removed in dry-run
	for _, f := range allFiles {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("Expected %s to be preserved in dry-run, but it was deleted", f)
		}
	}
	if _, err := os.Stat(tempFilesList); os.IsNotExist(err) {
		t.Errorf("Expected %s to be preserved in dry-run, but it was deleted", tempFilesList)
	}

	// Reset output buffer and test actual deletion
	outBuf.Reset()
	cleanDryRun = false
	err = cleanCmd.RunE(cleanCmd, []string{})
	if err != nil {
		t.Fatalf("cleanCmd failed: %v", err)
	}

	// Files that SHOULD be removed
	expectedRemoved := []string{file1, file2, tempFilesList, ephemeral1, ephemeral2, ephemeral3, ephemeral4}
	for _, f := range expectedRemoved {
		if _, err := os.Stat(f); !os.IsNotExist(err) {
			t.Errorf("Expected %s to be removed", f)
		}
	}

	// Files that SHOULD be kept
	expectedKept := []string{keep1, keep2, keep3, keep4}
	for _, f := range expectedKept {
		if _, err := os.Stat(f); os.IsNotExist(err) {
			t.Errorf("Expected %s to be preserved", f)
		}
	}
}
