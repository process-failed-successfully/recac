package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestScanForDependencyFiles(t *testing.T) {
	tmpDir := t.TempDir()

	files := []string{
		"go.mod",
		"package.json",
		"requirements.txt",
		"other.txt",
		"main.go",
		"Dockerfile",
		".github/workflows/ci.yml",
		".github/workflows/deploy.yaml",
		".github/README.md",
		"nested/go.mod", // Nested should not be found by shallow scan unless specified
	}

	for _, f := range files {
		path := filepath.Join(tmpDir, f)
		err := os.MkdirAll(filepath.Dir(path), 0755)
		require.NoError(t, err)
		err = os.WriteFile(path, []byte(""), 0644)
		require.NoError(t, err)
	}

	found, err := ScanForDependencyFiles(tmpDir)
	require.NoError(t, err)

	expected := []string{
		"go.mod",
		"package.json",
		"requirements.txt",
		"Dockerfile",
		".github/workflows/ci.yml",
		".github/workflows/deploy.yaml",
	}

	for _, e := range expected {
		path := filepath.Join(tmpDir, e)
		assert.Contains(t, found, path, "Expected to find %s", e)
	}

	assert.NotContains(t, found, filepath.Join(tmpDir, "other.txt"))
	assert.NotContains(t, found, filepath.Join(tmpDir, "main.go"))
	assert.NotContains(t, found, filepath.Join(tmpDir, ".github/README.md"))
	assert.NotContains(t, found, filepath.Join(tmpDir, "nested/go.mod"))
}

func TestGenerateRadarHTML(t *testing.T) {
	items := []RadarItem{
		{
			Name:        "Go",
			Quadrant:    "Languages & Frameworks",
			Ring:        "Adopt",
			Description: "The main language.",
		},
		{
			Name:        "Docker",
			Quadrant:    "Tools",
			Ring:        "Trial",
			Description: "Containerization.",
		},
	}

	html, err := GenerateRadarHTML(items)
	require.NoError(t, err)

	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "Technology Radar")
	assert.Contains(t, html, `{"name":"Go","quadrant":"Languages \u0026 Frameworks","ring":"Adopt","description":"The main language."}`)
	// JSON marshaling might escape & as \u0026, so checking for name is safer
	assert.Contains(t, html, `"name":"Go"`)
	assert.Contains(t, html, `"quadrant":"Languages \u0026 Frameworks"`)
	assert.Contains(t, html, `"ring":"Adopt"`)
}

func TestScanForDependencyFiles_Error(t *testing.T) {
	_, err := ScanForDependencyFiles("/nonexistent/directory/path/that/should/not/exist")
	require.Error(t, err)
}
