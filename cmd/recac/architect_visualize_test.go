package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchitectVisualize(t *testing.T) {
	// 1. Setup
	tmpDir := t.TempDir()
	archFile := filepath.Join(tmpDir, "architecture.yaml")
	archContent := `
system_name: TestSys
components:
  - id: api-gateway
    type: service
    consumes: []
    produces:
      - target: auth-service
        type: REST
  - id: auth-service
    type: service
    consumes:
      - source: api-gateway
        type: REST
    produces:
      - target: db
        type: SQL
  - id: db
    type: database
    consumes:
      - source: auth-service
        type: SQL
`
	err := os.WriteFile(archFile, []byte(archContent), 0644)
	require.NoError(t, err)

	// 2. Test Default (Stdout)
	// We need to ensure flags are reset
	resetFlags(rootCmd)

	output, err := executeCommand(rootCmd, "architect", "visualize", "--in", archFile)
	require.NoError(t, err)

	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "api_gateway -->|REST| auth_service")
	assert.Contains(t, output, "auth_service -->|SQL| db")
	assert.Contains(t, output, "style db fill:#e1f5fe") // Database style

	// 3. Test HTML Output
	outFile := filepath.Join(tmpDir, "graph.html")
	resetFlags(rootCmd)

	_, err = executeCommand(rootCmd, "architect", "visualize", "--in", archFile, "--out", outFile, "--html")
	require.NoError(t, err)

	// Check file exists
	htmlContent, err := os.ReadFile(outFile)
	require.NoError(t, err)
	assert.Contains(t, string(htmlContent), "<!DOCTYPE html>")
	assert.Contains(t, string(htmlContent), "mermaid.initialize")
	assert.Contains(t, string(htmlContent), "graph TD")
}
