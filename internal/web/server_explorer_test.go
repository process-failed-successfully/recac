package web

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestServer_HandleFiles(t *testing.T) {
	// Setup temp workspace
	tmpDir, err := os.MkdirTemp("", "web_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create dummy files
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("content1"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "subdir", "file2.txt"), []byte("content2"), 0644)

	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test", tmpDir)

	req, _ := http.NewRequest("GET", "/api/files", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(server.handleFiles)
	handler.ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)

	var tree []FileNode
	err = json.Unmarshal(rr.Body.Bytes(), &tree)
	assert.NoError(t, err)

	// Flatten for check
	foundFile1 := false
	foundDir := false

	for _, node := range tree {
		if node.Name == "file1.txt" {
			foundFile1 = true
		}
		if node.Name == "subdir" && node.Type == "dir" {
			foundDir = true
			if assert.Len(t, node.Children, 1) {
				assert.Equal(t, "file2.txt", node.Children[0].Name)
			}
		}
	}
	assert.True(t, foundFile1)
	assert.True(t, foundDir)
}

func TestServer_HandleFileContent(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "web_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	fPath := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(fPath, []byte("hello world"), 0644)

	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test", tmpDir)

	// Valid request
	req, _ := http.NewRequest("GET", "/api/file?path="+fPath, nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(server.handleFileContent).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "hello world", rr.Body.String())

	// Security check (path traversal)
	// Even if we point to a file outside, it should be blocked if we enforce workspace
	// But simple logic checks Prefix.

	// Try accessing a file outside
	outsidePath := filepath.Join(tmpDir, "..", "outside.txt")
	req, _ = http.NewRequest("GET", "/api/file?path="+outsidePath, nil)
	rr = httptest.NewRecorder()
	http.HandlerFunc(server.handleFileContent).ServeHTTP(rr, req)
	// It should fail either 403 or 404 (if Abs fails weirdly)
	assert.NotEqual(t, http.StatusOK, rr.Code)
}

func TestServer_HandleFileDiagram(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "web_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	code := `package main
	type Foo struct {}
	`
	fPath := filepath.Join(tmpDir, "main.go")
	os.WriteFile(fPath, []byte(code), 0644)

	mockStore := &MockStore{}
	server := NewServer(mockStore, 8080, "test", tmpDir)

	req, _ := http.NewRequest("GET", "/api/diagram?path="+fPath, nil)
	rr := httptest.NewRecorder()
	http.HandlerFunc(server.handleFileDiagram).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	assert.Contains(t, rr.Body.String(), "classDiagram")
	assert.Contains(t, rr.Body.String(), "class main_Foo")
}
