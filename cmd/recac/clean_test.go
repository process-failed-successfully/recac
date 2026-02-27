package main

import (
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

	// Create dummy files to be cleaned
	file1 := "file1.txt"
	file2 := "subdir/file2.txt"

	if err := os.MkdirAll(filepath.Dir(file2), 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	if err := os.WriteFile(file1, []byte("content1"), 0644); err != nil {
		t.Fatalf("Failed to write file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content2"), 0644); err != nil {
		t.Fatalf("Failed to write file2: %v", err)
	}

	// Create temp_files.txt listing these files
	// The clean command expects paths. It resolves them using filepath.Abs.
	// We can put relative paths in the file as the command runs from the same CWD.
	tempFilesList := "temp_files.txt"
	content := file1 + "\n" + file2 + "\n"
	if err := os.WriteFile(tempFilesList, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp_files.txt: %v", err)
	}

	// Run clean command
	// We can invoke Run directly
	cleanCmd.Run(cleanCmd, []string{})

	// Verify files are removed
	if _, err := os.Stat(file1); !os.IsNotExist(err) {
		t.Errorf("Expected %s to be removed", file1)
	}
	if _, err := os.Stat(file2); !os.IsNotExist(err) {
		t.Errorf("Expected %s to be removed", file2)
	}

	// Verify temp_files.txt is removed
	if _, err := os.Stat(tempFilesList); !os.IsNotExist(err) {
		t.Errorf("Expected %s to be removed", tempFilesList)
	}
}
