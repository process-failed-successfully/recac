package main

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestCreateTarStream(t *testing.T) {
	// Create a temporary directory for build context
	tmpDir, err := os.MkdirTemp("", "recac-build-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some files
	files := map[string]string{
		"Dockerfile":       "FROM alpine\nRUN echo hello",
		"main.go":          "package main",
		"src/utils.go":     "package utils",
		".git/config":      "git config", // Should be ignored
		"symlink":          "main.go",    // specific handling
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		if path == "symlink" {
			// Symlink handling
			if err := os.Symlink("main.go", fullPath); err != nil {
				t.Fatal(err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}

	// Call createTarStream
	reader, err := createTarStream(tmpDir)
	if err != nil {
		t.Fatalf("createTarStream failed: %v", err)
	}

	// Verify tar content
	tr := tar.NewReader(reader)
	foundFiles := make(map[string]bool)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}

		foundFiles[header.Name] = true

		if header.Name == "Dockerfile" {
			content, _ := io.ReadAll(tr)
			if string(content) != files["Dockerfile"] {
				t.Errorf("Dockerfile content mismatch")
			}
		}

		if header.Typeflag == tar.TypeSymlink {
			if header.Linkname != "main.go" {
				t.Errorf("Symlink target mismatch. Got %s, want main.go", header.Linkname)
			}
		}
	}

	// Assertions
	if !foundFiles["Dockerfile"] {
		t.Error("Dockerfile not found in tar")
	}
	if !foundFiles["main.go"] {
		t.Error("main.go not found in tar")
	}
	// "src/utils.go" -> tar paths are usually cleaned, let's check exact name
	// filepath.Rel might return "src/utils.go" (OS dependent separator)
	// tar uses forward slashes usually?
	// The function uses `filepath.Rel` which uses OS separator.
	// But `tar.Header.Name` should be forward slash for portability?
	// The function sets `header.Name = relPath`.
	// On Linux it's fine.

	srcKey := "src/utils.go"
	if !foundFiles[srcKey] {
		t.Errorf("%s not found in tar", srcKey)
	}

	if foundFiles[".git/config"] {
		t.Error(".git/config should be ignored")
	}

	if !foundFiles["symlink"] {
		t.Error("symlink not found in tar")
	}
}
