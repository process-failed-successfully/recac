package main

import (
	"os"
	"path/filepath"
	"testing"

	"recac/internal/architecture"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeArchID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"User-Service", "User_Service"},
		{"api.v1", "api_v1"},
		{"db_1", "db_1"},
		{"complex/id-test", "complex_id_test"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.expected, sanitizeArchID(tt.input))
		})
	}
}

func TestEscapeMermaidLabel(t *testing.T) {
	input := `User "Profile"`
	expected := `User 'Profile'`
	assert.Equal(t, expected, escapeMermaidLabel(input))
}

func TestGenerateMermaidSystemArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{
				ID:   "frontend",
				Type: "frontend",
				Consumes: []architecture.Input{
					{Source: "backend", Type: "http"},
				},
			},
			{
				ID:   "backend",
				Type: "service",
				Produces: []architecture.Output{
					{Target: "db", Type: "sql"},
				},
			},
			{
				ID:   "db",
				Type: "database",
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Check for nodes
	assert.Contains(t, mermaid, `frontend["frontend<br/>(frontend)"]`)
	assert.Contains(t, mermaid, `backend["backend<br/>(service)"]`)
	assert.Contains(t, mermaid, `db["db<br/>(database)"]`)

	// Check for styles
	assert.Contains(t, mermaid, "class frontend frontend;")
	assert.Contains(t, mermaid, "class backend service;")
	assert.Contains(t, mermaid, "class db database;")

	// Check for edges
	assert.Contains(t, mermaid, `backend -- "http" --> frontend`)
	assert.Contains(t, mermaid, `backend -- "sql" --> db`)
}

func TestGenerateHTML(t *testing.T) {
	tempDir := t.TempDir()
	path := filepath.Join(tempDir, "test.html")
	mermaid := "graph TD; A-->B;"

	err := generateHTML(path, mermaid)
	require.NoError(t, err)

	content, err := os.ReadFile(path)
	require.NoError(t, err)

	html := string(content)
	assert.Contains(t, html, "<html>")
	assert.Contains(t, html, mermaid)
	assert.Contains(t, html, "mermaid.min.js")
}

func TestArchitectVisualizeCmd(t *testing.T) {
	// Setup temp dir with architecture.yaml
	tempDir := t.TempDir()
	archContent := `
system_name: TestSystem
version: "1.0"
components:
  - id: api
    type: service
`
	archDir := filepath.Join(tempDir, ".recac/architecture")
	err := os.MkdirAll(archDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(archDir, "architecture.yaml"), []byte(archContent), 0644)
	require.NoError(t, err)

	cmd := architectVisualizeCmd

	// Test writing to file
	outFile := filepath.Join(tempDir, "output.mmd")

	// Manually set flags
	cmd.Flags().Set("dir", archDir)
	cmd.Flags().Set("out", outFile)
	cmd.Flags().Set("html", "false")

	err = cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	require.FileExists(t, outFile)
	content, _ := os.ReadFile(outFile)
	assert.Contains(t, string(content), "graph TD")
	assert.Contains(t, string(content), "api")

    // Test HTML output
    htmlOut := filepath.Join(tempDir, "output.html")
    cmd.Flags().Set("out", htmlOut)
    cmd.Flags().Set("html", "true")

    err = cmd.RunE(cmd, []string{})
    require.NoError(t, err)

    require.FileExists(t, htmlOut)
    contentHTML, _ := os.ReadFile(htmlOut)
    assert.Contains(t, string(contentHTML), "<!DOCTYPE html>")
    assert.Contains(t, string(contentHTML), "api")
}
