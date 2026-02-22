package main

import (
	"os"
	"path/filepath"
	"testing"

	"recac/internal/architecture"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMermaidFromArch(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		SystemName: "TestSystem",
		Components: []architecture.Component{
			{
				ID:   "web-server",
				Type: "service",
				Consumes: []architecture.Input{
					{Source: "user-client", Type: "http"},
				},
				Produces: []architecture.Output{
					{Target: "database", Type: "sql"},
				},
			},
			{
				ID:   "database",
				Type: "database",
			},
			{
				ID:   "user-client",
				Type: "frontend",
			},
		},
	}

	mermaid := generateMermaidFromArch(arch)

	// Check for nodes
	assert.Contains(t, mermaid, "web_server[")
	assert.Contains(t, mermaid, "database[(") // database shape
	assert.Contains(t, mermaid, "user_client{{") // frontend shape

	// Check for edges
	// user-client -> web-server
	assert.Contains(t, mermaid, "user_client -->|http| web_server")
	// web-server -> database
	assert.Contains(t, mermaid, "web_server -->|sql| database")
}

func TestArchitectVisualizeCmd(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-arch-viz-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a valid architecture.yaml
	archContent := `
system_name: TestSystem
components:
  - id: service-a
    type: service
    produces:
      - target: service-b
        type: grpc
  - id: service-b
    type: service
`
	archFile := filepath.Join(tmpDir, "architecture.yaml")
	require.NoError(t, os.WriteFile(archFile, []byte(archContent), 0644))

	t.Run("Generate Text Output", func(t *testing.T) {
		outFile := filepath.Join(tmpDir, "output.mermaid")

		// Use root command to simulate full execution path
		rootCmd.SetArgs([]string{
			"architect", "visualize",
			"--arch-file", archFile,
			"--out", outFile,
		})

		// Execute
		err := rootCmd.Execute()
		require.NoError(t, err)

		// Verify
		content, err := os.ReadFile(outFile)
		require.NoError(t, err)
		sContent := string(content)
		assert.Contains(t, sContent, "graph TD")
		assert.Contains(t, sContent, "service_a -->|grpc| service_b")
	})

	t.Run("Generate HTML Output", func(t *testing.T) {
		outFile := filepath.Join(tmpDir, "output.html")

		rootCmd.SetArgs([]string{
			"architect", "visualize",
			"--arch-file", archFile,
			"--out", outFile,
			"--html",
		})

		err := rootCmd.Execute()
		require.NoError(t, err)

		content, err := os.ReadFile(outFile)
		require.NoError(t, err)
		sContent := string(content)
		assert.Contains(t, sContent, "<!DOCTYPE html>")
		assert.Contains(t, sContent, "mermaid.initialize")
		assert.Contains(t, sContent, "service_a -->|grpc| service_b")
	})
}
