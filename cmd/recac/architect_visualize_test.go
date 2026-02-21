package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchitectVisualize(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-arch-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create dummy architecture.yaml
	archContent := `
system_name: "TestSystem"
components:
  - id: "ServiceA"
    type: "service"
    consumes:
      - source: "ServiceB"
        type: "event"
    produces:
      - target: "Database1"
        type: "data"

  - id: "ServiceB"
    type: "worker"

  - id: "Database1"
    type: "database"
`
	err = os.WriteFile(filepath.Join(tmpDir, "architecture.yaml"), []byte(archContent), 0644)
	require.NoError(t, err)

	// Test 1: Default output (Mermaid)
	output, err := executeCommand(rootCmd, "architect", "visualize", "--dir", tmpDir)
	require.NoError(t, err)

	// Assert output contains Mermaid syntax
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "ServiceA")
	assert.Contains(t, output, "ServiceB")
	assert.Contains(t, output, "Database1")
	// Check shapes - loose matching to avoid spacing issues
	assert.Contains(t, output, "Database1[(")
	assert.Contains(t, output, "ServiceB{{")
	// Check edges
	assert.Contains(t, output, "ServiceB -- \"event\" --> ServiceA")
	assert.Contains(t, output, "ServiceA -- \"data\" --> Database1")

	// Test 2: HTML Output
	output, err = executeCommand(rootCmd, "architect", "visualize", "--dir", tmpDir, "--html")
	require.NoError(t, err)

	assert.Contains(t, output, "Generated architecture visualization at")

	htmlPath := filepath.Join(tmpDir, "architecture.html")
	require.FileExists(t, htmlPath)

	htmlContent, err := os.ReadFile(htmlPath)
	require.NoError(t, err)

	assert.Contains(t, string(htmlContent), "<!DOCTYPE html>")
	assert.Contains(t, string(htmlContent), "mermaid.initialize")
	assert.Contains(t, string(htmlContent), "graph TD")
}
