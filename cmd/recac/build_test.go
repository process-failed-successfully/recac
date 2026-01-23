package main

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateTarStream(t *testing.T) {
	// Setup build context
	tmpDir := t.TempDir()

	// Create some files
	os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte("FROM alpine"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644)
	os.Mkdir(filepath.Join(tmpDir, "src"), 0755)
	os.WriteFile(filepath.Join(tmpDir, "src/lib.go"), []byte("package lib"), 0644)

	// Create .git dir (should be ignored)
	os.Mkdir(filepath.Join(tmpDir, ".git"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".git/config"), []byte("config"), 0644)

	// Execute
	reader, err := createTarStream(tmpDir)
	assert.NoError(t, err)
	assert.NotNil(t, reader)

	// Verify tar content
	tr := tar.NewReader(reader)

	filesFound := make(map[string]bool)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		assert.NoError(t, err)
		filesFound[header.Name] = true
	}

	assert.True(t, filesFound["Dockerfile"], "Dockerfile should be in tar")
	assert.True(t, filesFound["main.go"], "main.go should be in tar")
	assert.True(t, filesFound["src/lib.go"], "src/lib.go should be in tar")
	assert.False(t, filesFound[".git/config"], ".git should be ignored")
	assert.False(t, filesFound[".git"], ".git dir should be ignored")
}
