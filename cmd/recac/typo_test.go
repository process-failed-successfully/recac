package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractTypoCandidates(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "test.go")
	content := `package main
// This contains a typo: reciever
func main() {
	var val string = "value"
}
`
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	candidates, fileMap := extractTypoCandidates([]string{file})

	found := false
	for _, c := range candidates {
		if c == "reciever" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected 'reciever' in candidates")
	}

	files := fileMap["reciever"]
	if len(files) != 1 || files[0] != file {
		t.Errorf("Expected file map to point to test file for 'reciever', got %v", files)
	}
}

func TestReplaceInFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "replace.txt")
	content := "This has a typo: tehh"
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}

	if err := replaceInFile(file, "tehh", "the"); err != nil {
		t.Fatalf("replaceInFile failed: %v", err)
	}

	newContent, err := os.ReadFile(file)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(newContent) != "This has a typo: the" {
		t.Errorf("Expected replaced content, got '%s'", string(newContent))
	}
}

func TestScanFilesForTypo(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")
	os.WriteFile(file1, []byte("text"), 0644)
	os.WriteFile(file2, []byte("text"), 0644)

	// Create binary file (should be skipped)
	binFile := filepath.Join(tmpDir, "image.png")
	os.WriteFile(binFile, []byte{0x89, 0x50, 0x4E, 0x47}, 0644)

	files, err := scanFilesForTypo(tmpDir, 10)
	if err != nil {
		t.Fatalf("scanFilesForTypo failed: %v", err)
	}

	found1 := false
	found2 := false
	foundBin := false

	for _, f := range files {
		if f == file1 { found1 = true }
		if f == file2 { found2 = true }
		if f == binFile { foundBin = true }
	}

	if !found1 || !found2 {
		t.Error("Expected text files to be found")
	}
	if foundBin {
		t.Error("Expected binary file to be skipped")
	}
}

func TestFindFilesWithWord(t *testing.T) {
	fileMap := map[string][]string{
		"word": {"file1", "file2"},
	}
	files := findFilesWithWord(fileMap, "word")
	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}
}
