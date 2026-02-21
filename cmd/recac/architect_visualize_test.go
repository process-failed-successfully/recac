package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchitectVisualize(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "recac-arch-viz-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a sample architecture.yaml
	archContent := `
system_name: "Test System"
components:
  - id: "Service A"
    type: "service"
    consumes:
      - source: "Service B"
        type: "gRPC"
    produces:
      - target: "Database"
        type: "SQL"
  - id: "Service B"
    type: "service"
  - id: "Database"
    type: "database"
`
	err = os.WriteFile(filepath.Join(tmpDir, "architecture.yaml"), []byte(archContent), 0644)
	require.NoError(t, err)

	t.Run("Generate Mermaid to Stdout", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "architect", "visualize", "--dir", tmpDir)
		require.NoError(t, err)

		// Check for key Mermaid elements
		assert.Contains(t, output, "flowchart TD")
		// IDs are now hashed, so we check for labels and structure
		assert.Contains(t, output, `["Service A<br/>(service)"]`)
		assert.Contains(t, output, `-->|gRPC|`)
		assert.Contains(t, output, `-->|SQL|`)
	})

	t.Run("Generate HTML file", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "architect", "visualize", "--dir", tmpDir, "--html")
		require.NoError(t, err)

		// Check output message
		assert.Contains(t, output, "Architecture diagram saved to")
		assert.Contains(t, output, "architecture.html")

		// Check if file exists
		htmlPath := filepath.Join(tmpDir, "architecture.html")
		content, err := os.ReadFile(htmlPath)
		require.NoError(t, err)

		htmlStr := string(content)
		assert.Contains(t, htmlStr, "<!DOCTYPE html>")
		assert.Contains(t, htmlStr, "mermaid.initialize")
		assert.Contains(t, htmlStr, "flowchart TD")
		assert.Contains(t, htmlStr, `-->|SQL|`)
	})

	t.Run("Missing File", func(t *testing.T) {
		emptyDir, err := os.MkdirTemp("", "recac-arch-viz-empty")
		require.NoError(t, err)
		defer os.RemoveAll(emptyDir)

		_, err = executeCommand(rootCmd, "architect", "visualize", "--dir", emptyDir)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to read architecture file")
	})
}
