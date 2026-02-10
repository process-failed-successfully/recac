package main

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestLinksCmd(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()

	// Create structure:
	// doc.md: [valid](target.md), [broken](missing.md), [anchor](target.md#header)
	// target.md

	docContent := `
# Doc
[Valid Link](target.md)
[Broken Link](missing.md)
[Anchor Link](target.md#header)
[Self Anchor](#header)
`
	err := os.WriteFile(filepath.Join(tmpDir, "doc.md"), []byte(docContent), 0644)
	assert.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "target.md"), []byte("Target"), 0644)
	assert.NoError(t, err)

	// Create a dummy command to capture output
	var outBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	// 1. Run Scan (no fix)
	// Expect error because missing.md is broken
	linksFix = false
	err = runLinks(cmd, []string{tmpDir})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "found 1 broken links")

	// 2. Test Fix Logic
	// We will create a scenario where a file moved.
	// doc_fix.md -> links to old_file.md
	// but old_file.md is moved to sub/new_file.md (same name different dir? or different name?)
	// My logic only supports same name currently.
	// So let's test: links to "moved.md", but file is at "sub/moved.md"

	os.Mkdir(filepath.Join(tmpDir, "sub"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "sub", "moved.md"), []byte("Moved content"), 0644)

	docFixContent := `[Moved Link](moved.md)`
	os.WriteFile(filepath.Join(tmpDir, "doc_fix.md"), []byte(docFixContent), 0644)

	// Enable fix
	linksFix = true
	defer func() { linksFix = false }()

	// Capture output again
	outBuf.Reset()

	// Run
	err = runLinks(cmd, []string{tmpDir})
	// It might still return error if "missing.md" is still broken (from doc.md)
	// doc.md has missing.md which is NOT fixed.
	// doc_fix.md has moved.md which SHOULD be fixed.
	assert.Error(t, err) // Still have broken links

	// Check output
	output := outBuf.String()
	t.Log(output)

	// Check if doc_fix.md was updated
	newContent, err := os.ReadFile(filepath.Join(tmpDir, "doc_fix.md"))
	assert.NoError(t, err)
	assert.Contains(t, string(newContent), "(sub/moved.md)", "Link should be updated to point to sub/moved.md")

	// 3. Test Anchor Preservation during fix
	// Create doc_anchor.md -> [Link](anchored.md#section)
	// File is at sub/anchored.md
	os.WriteFile(filepath.Join(tmpDir, "sub", "anchored.md"), []byte("Anchored"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "doc_anchor.md"), []byte("[Link](anchored.md#section)"), 0644)

	outBuf.Reset()
	runLinks(cmd, []string{tmpDir})

	newAnchorContent, _ := os.ReadFile(filepath.Join(tmpDir, "doc_anchor.md"))
	assert.Contains(t, string(newAnchorContent), "(sub/anchored.md#section)", "Link should update path but preserve anchor")
}

func TestLinksCmd_External(t *testing.T) {
	// Restore original httpHeadFunc after test
	originalHttpHeadFunc := httpHeadFunc
	defer func() { httpHeadFunc = originalHttpHeadFunc }()

	// Setup mock
	mockResponses := map[string]int{
		"http://valid.com":   200,
		"http://broken.com":  404,
		"http://error.com":   0, // simulates error
	}

	httpHeadFunc = func(url string) (*http.Response, error) {
		code, ok := mockResponses[url]
		if !ok {
			return nil, errors.New("unknown url")
		}
		if code == 0 {
			return nil, errors.New("network error")
		}
		return &http.Response{
			StatusCode: code,
			Body:       io.NopCloser(bytes.NewReader(nil)),
		}, nil
	}

	// Setup fs
	tmpDir := t.TempDir()
	content := `
[Valid](http://valid.com)
[Broken](http://broken.com)
[Error](http://error.com)
[Ignored](http://unknown.com)
`
	err := os.WriteFile(filepath.Join(tmpDir, "external.md"), []byte(content), 0644)
	assert.NoError(t, err)

	// Run
	var outBuf bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	linksExternal = true
	defer func() { linksExternal = false }()

	err = runLinks(cmd, []string{tmpDir})
	assert.Error(t, err) // broken links found

	output := outBuf.String()
	assert.Contains(t, output, "http://broken.com")
	assert.Contains(t, output, "http://error.com") // checkURL returns false on error, so it appears as broken
	assert.NotContains(t, output, "http://valid.com")
}

func TestLinksCmd_Errors(t *testing.T) {
	// 1. Invalid root
	cmd := &cobra.Command{}
	var outBuf bytes.Buffer
	cmd.SetOut(&outBuf)
	cmd.SetErr(&outBuf)

	err := runLinks(cmd, []string{"/non/existent/path"})
	assert.Error(t, err)

	// 2. Write error during fix
	tmpDir := t.TempDir()

	// Prepare files
	os.Mkdir(filepath.Join(tmpDir, "sub"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "sub", "target.md"), []byte("Target"), 0644)
	docPath := filepath.Join(tmpDir, "doc.md")
	os.WriteFile(docPath, []byte("[Link](target.md)"), 0644) // target.md missing in root, exists in sub

	// Mock writeFileFunc to fail
	originalWriteFileFunc := writeFileFunc
	defer func() { writeFileFunc = originalWriteFileFunc }()

	writeFileFunc = func(filename string, data []byte, perm os.FileMode) error {
		return errors.New("write failed")
	}

	linksFix = true
	defer func() { linksFix = false }()

	err = runLinks(cmd, []string{tmpDir})
	assert.Error(t, err)
	// Output should mention write failure
	assert.Contains(t, outBuf.String(), "Failed to update")
}
