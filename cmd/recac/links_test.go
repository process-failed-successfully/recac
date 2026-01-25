package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
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

func TestCheckURL(t *testing.T) {
	// Setup mock server
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusOK)
		} else if r.URL.Path == "/404" {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer ts.Close()

	assert.True(t, checkURL(ts.URL+"/ok"))
	assert.False(t, checkURL(ts.URL+"/404"))
	assert.False(t, checkURL(ts.URL+"/error"))
	assert.False(t, checkURL("http://invalid-url-that-does-not-exist"))
}

func TestScanLinks_External(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ts.Close()

	tmpDir := t.TempDir()
	content := "[Valid External](" + ts.URL + "/ok)\n[Broken External](" + ts.URL + "/404)"
	err := os.WriteFile(filepath.Join(tmpDir, "external.md"), []byte(content), 0644)
	assert.NoError(t, err)

	// Test with external check enabled
	broken, err := scanLinks(tmpDir, true)
	assert.NoError(t, err)

	// Should find 1 broken link
	foundBroken := false
	for _, bl := range broken {
		if bl.IsExternal && bl.Target == ts.URL+"/404" {
			foundBroken = true
		}
	}
	assert.True(t, foundBroken, "Should have found broken external link")

	// Test with external check disabled
	broken, err = scanLinks(tmpDir, false)
	assert.NoError(t, err)
	assert.Len(t, broken, 0, "Should not find any broken links when external check is disabled")
}
