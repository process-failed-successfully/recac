package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/architecture"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMermaidSystemArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		SystemName: "Test System",
		Components: []architecture.Component{
			{
				ID:   "API Service",
				Type: "service",
				Produces: []architecture.Output{
					{Target: "Worker", Type: "Job"},
				},
			},
			{
				ID:   "Worker",
				Type: "worker",
				Consumes: []architecture.Input{
					{Source: "API Service", Type: "Job"},
				},
				Produces: []architecture.Output{
					{Target: "Database", Type: "Result"},
				},
			},
			{
				ID:   "Database",
				Type: "database",
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Verify nodes
	assert.Contains(t, mermaid, "API_Service[\"API Service\"]")
	assert.Contains(t, mermaid, "Worker{{ \"Worker\" }}")
	assert.Contains(t, mermaid, "Database[(\"Database\")]")

	// Verify edges
	assert.Contains(t, mermaid, "API_Service -->|Job| Worker")
}

func TestSanitizeArchID(t *testing.T) {
	assert.Equal(t, "My_Service", sanitizeArchID("My Service"))
	assert.Equal(t, "foo_bar", sanitizeArchID("foo/bar"))
	assert.Equal(t, "a_b_c", sanitizeArchID("a.b.c"))
}

func TestEscapeMermaidLabel(t *testing.T) {
	assert.Equal(t, "Hello #quot;World#quot;", escapeMermaidLabel("Hello \"World\""))
	assert.Equal(t, "Tags #lt;xml#gt;", escapeMermaidLabel("Tags <xml>"))
}

func TestGenerateHTML(t *testing.T) {
	// Create a temp dir
	tmpDir := t.TempDir()
	archFile := filepath.Join(tmpDir, "architecture.yaml")

	yamlContent := `
system_name: TestSys
components:
  - id: Web
    type: frontend
  - id: API
    type: service
    consumes:
      - source: Web
        type: HTTP
`
	err := os.WriteFile(archFile, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Create a command instance for testing
	cmd := &cobra.Command{
		RunE: runArchitectVisualizeCmd,
	}
	// Define flags as in real command
	cmd.Flags().String("dir", ".recac/architecture", "")
	cmd.Flags().String("out", "", "")
	cmd.Flags().Bool("html", false, "")

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Set flags
	outFile := filepath.Join(tmpDir, "test.html")
	cmd.SetArgs([]string{"--dir", tmpDir, "--html", "--out", outFile})

	// Run
	err = cmd.Execute()
	require.NoError(t, err)

	// Verify file
	require.FileExists(t, outFile)
	content, err := os.ReadFile(outFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "<!DOCTYPE html>")
	assert.Contains(t, string(content), "mermaid.initialize")
	assert.Contains(t, string(content), "Web([\"Web\"])")
}
