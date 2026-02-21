package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunArchitectVisualize(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Sample architecture.yaml content
	archContent := `
version: "1.0"
system_name: "TestSystem"
components:
  - id: "service-a"
    type: "service"
    produces:
      - target: "service-b"
        event: "EventA"
  - id: "service-b"
    type: "worker"
    consumes:
      - source: "service-a"
        type: "EventA"
`
	archPath := filepath.Join(tmpDir, "architecture.yaml")
	err := os.WriteFile(archPath, []byte(archContent), 0644)
	require.NoError(t, err)

	// Test Case 1: Default output (Mermaid to stdout)
	t.Run("DefaultOutput", func(t *testing.T) {
		cmd := architectVisualizeCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		// Reset flags
		cmd.Flags().Set("dir", tmpDir)
		cmd.Flags().Set("html", "false")

		err := runArchitectVisualize(cmd, []string{})
		require.NoError(t, err)

		output := buf.String()
		assert.Contains(t, output, "graph TD")
		assert.Contains(t, output, "service_a")
		assert.Contains(t, output, "service_b")
		assert.Contains(t, output, "EventA")
	})

	// Test Case 2: HTML output
	t.Run("HTMLOutput", func(t *testing.T) {
		cmd := architectVisualizeCmd
		buf := new(bytes.Buffer)
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		// Reset flags
		cmd.Flags().Set("dir", tmpDir)
		cmd.Flags().Set("html", "true")

		err := runArchitectVisualize(cmd, []string{})
		require.NoError(t, err)

		htmlPath := filepath.Join(tmpDir, "architecture.html")
		assert.FileExists(t, htmlPath)

		htmlContent, err := os.ReadFile(htmlPath)
		require.NoError(t, err)

		htmlStr := string(htmlContent)
		assert.Contains(t, htmlStr, "<!DOCTYPE html>")
		assert.Contains(t, htmlStr, "mermaid.min.js")
		assert.Contains(t, htmlStr, "graph TD")
		assert.Contains(t, htmlStr, "service_a")
	})
}
