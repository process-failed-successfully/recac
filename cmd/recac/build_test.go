package main

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTarStream(t *testing.T) {
	// 1. Create temporary directory structure
	tmpDir := t.TempDir()

	// Add a regular file
	file1Path := filepath.Join(tmpDir, "file1.txt")
	err := os.WriteFile(file1Path, []byte("content1"), 0644)
	require.NoError(t, err)

	// Add a subdirectory with a file
	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	file2Path := filepath.Join(subDir, "file2.txt")
	err = os.WriteFile(file2Path, []byte("content2"), 0644)
	require.NoError(t, err)

	// Add a file to ignore (.git directory)
	gitDir := filepath.Join(tmpDir, ".git")
	err = os.Mkdir(gitDir, 0755)
	require.NoError(t, err)
	gitFile := filepath.Join(gitDir, "config")
	err = os.WriteFile(gitFile, []byte("gitconfig"), 0644)
	require.NoError(t, err)

	// 2. Call createTarStream
	reader, err := createTarStream(tmpDir)
	require.NoError(t, err)

	// 3. Verify Tar Content
	tr := tar.NewReader(reader)

	filesFound := make(map[string]string)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		require.NoError(t, err)

		if header.Typeflag == tar.TypeReg {
			content, err := io.ReadAll(tr)
			require.NoError(t, err)
			filesFound[header.Name] = string(content)
		}
	}

	// file1.txt should exist
	assert.Contains(t, filesFound, "file1.txt")
	assert.Equal(t, "content1", filesFound["file1.txt"])

	// subdir/file2.txt should exist
	// Note: filepath.Walk uses OS separator. On Windows it might be \, but tar uses /?
	// createTarStream uses filepath.Rel which preserves OS separator.
	// We might need to handle separator differences if testing cross-platform strictly,
	// but normally tar headers use forward slashes.
	// Let's check for both or normalize.

	// Assuming unix-like paths in tar or handled by createTarStream logic (which currently just uses Rel path)
	// If createTarStream puts OS-specific paths in header.Name, that's what we get.

	key2 := filepath.Join("subdir", "file2.txt")
	assert.Contains(t, filesFound, key2)
	assert.Equal(t, "content2", filesFound[key2])

	// .git content should NOT exist
	assert.NotContains(t, filesFound, filepath.Join(".git", "config"))
}
