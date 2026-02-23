package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestScanFilesForTypo(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a text file
	txtFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(txtFile, []byte("some content"), 0644)
	assert.NoError(t, err)

	// Create a go file
	goFile := filepath.Join(tmpDir, "main.go")
	err = os.WriteFile(goFile, []byte("package main"), 0644)
	assert.NoError(t, err)

	// Create a binary file (simulate by extension or content?)
	// typo.go uses utils.IsBinaryExt. Let's use a known binary extension like .png
	binFile := filepath.Join(tmpDir, "image.png")
	err = os.WriteFile(binFile, []byte("fake binary"), 0644)
	assert.NoError(t, err)

	// Create ignored directory
	gitDir := filepath.Join(tmpDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	assert.NoError(t, err)
	gitFile := filepath.Join(gitDir, "HEAD")
	err = os.WriteFile(gitFile, []byte("ref: refs/heads/main"), 0644)
	assert.NoError(t, err)

	files, err := scanFilesForTypo(tmpDir, 10)
	assert.NoError(t, err)

	// Should contain test.txt and main.go, but not image.png or .git/HEAD
	assert.Contains(t, files, txtFile)
	assert.Contains(t, files, goFile)
	assert.NotContains(t, files, binFile)
	assert.NotContains(t, files, gitFile)
}

func TestExtractTypoCandidates(t *testing.T) {
	tmpDir := t.TempDir()
	file1 := filepath.Join(tmpDir, "file1.txt")
	// "reciever" is a common typo for "receiver". "func" is allowed. "myVar" is mixed case.
	content1 := "func myFunction() { // reciever\n var myVar = 1 }"
	os.WriteFile(file1, []byte(content1), 0644)

	files := []string{file1}
	candidates, fileMap := extractTypoCandidates(files)

	// candidates should contain "reciever", "myVar"?
	// logic: alpha only, min 4 chars.
	// "func" -> allowed
	// "myFunction" -> candidate?
	// "reciever" -> candidate
	// "myVar" -> candidate (5 chars)

	// Check if "reciever" is present
	found := false
	for _, c := range candidates {
		if c == "reciever" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected 'reciever' in candidates")

	// Check file map
	assert.Contains(t, fileMap["reciever"], file1)

	// Check allowlist
	// "func" should not be in candidates
	foundFunc := false
	for _, c := range candidates {
		if c == "func" {
			foundFunc = true
			break
		}
	}
	assert.False(t, foundFunc, "Expected 'func' to be ignored")
}

func TestReplaceInFile(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "fix.txt")
	content := "This has a reciever typo. repeated reciever."
	os.WriteFile(file, []byte(content), 0644)

	err := replaceInFile(file, "reciever", "receiver")
	assert.NoError(t, err)

	newContent, err := os.ReadFile(file)
	assert.NoError(t, err)
	assert.Equal(t, "This has a receiver typo. repeated receiver.", string(newContent))
}

func TestReplaceInFile_PartialMatch(t *testing.T) {
	tmpDir := t.TempDir()
	file := filepath.Join(tmpDir, "partial.txt")
	// "model" should not replace "nodel" if we are replacing "mode"?
	// Wait, typo.go uses \b boundaries.
	// Let's test: replace "cat" with "dog". "concatenate" should stay "concatenate".
	content := "The cat sat. concatenate."
	os.WriteFile(file, []byte(content), 0644)

	err := replaceInFile(file, "cat", "dog")
	assert.NoError(t, err)

	newContent, err := os.ReadFile(file)
	assert.NoError(t, err)
	assert.Equal(t, "The dog sat. concatenate.", string(newContent))
}
