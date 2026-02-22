package main

import (
	"os"
	"path/filepath"
	"testing"

	"recac/internal/architecture"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMermaidArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		SystemName: "TestSystem",
		Components: []architecture.Component{
			{
				ID:   "api-service",
				Type: "service",
				Consumes: []architecture.Input{
					{Source: "frontend-app", Type: "http"},
				},
				Produces: []architecture.Output{
					{Target: "db-primary", Type: "sql"},
				},
			},
			{
				ID:   "db-primary",
				Type: "database",
			},
			{
				ID:   "frontend-app",
				Type: "frontend",
			},
		},
	}

	mermaid := generateMermaidArchitecture(arch)

	// Check graph definition
	assert.Contains(t, mermaid, "graph TD")

	// Check nodes exist and have correct shapes
	// api-service -> default rect
	assert.Contains(t, mermaid, "\"api-service\"]")
	// db-primary -> cylinder [()
	assert.Contains(t, mermaid, "[(\"db-primary\")]")
	// frontend-app -> circle (())
	assert.Contains(t, mermaid, "((\"frontend-app\"))")

	// Check edges
	// frontend -> api
	assert.Contains(t, mermaid, "-- \"http\" -->")
	// api -> db
	assert.Contains(t, mermaid, "-- \"sql\" -->")

	// Check external node handling
	archWithExternal := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{
				ID:   "worker",
				Type: "worker",
				Consumes: []architecture.Input{
					{Source: "external-queue", Type: "amqp"},
				},
			},
		},
	}
	mermaidExt := generateMermaidArchitecture(archWithExternal)
	assert.Contains(t, mermaidExt, "\"external-queue (External)\"]:::external")
}

func TestVisualizeCmd_Integration(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	archDir := filepath.Join(tmpDir, ".recac", "architecture")
	err := os.MkdirAll(archDir, 0755)
	require.NoError(t, err)

	archFile := filepath.Join(archDir, "architecture.yaml")
	content := `
system_name: IntegrationTest
components:
  - id: web
    type: service
  - id: db
    type: database
    consumes:
      - source: web
        type: queries
`
	err = os.WriteFile(archFile, []byte(content), 0644)
	require.NoError(t, err)

	outFile := filepath.Join(tmpDir, "output.mermaid")

	// Manually set flag and invoke RunE
	cmd := visualizeCmd
	err = cmd.Flags().Set("out", outFile)
	require.NoError(t, err)

	// Invoke logic directly
	err = runVisualizeCmd(cmd, []string{archFile})
	require.NoError(t, err)

	// Verify output file
	outContent, err := os.ReadFile(outFile)
	require.NoError(t, err)
	assert.Contains(t, string(outContent), "graph TD")
	assert.Contains(t, string(outContent), "web")
	assert.Contains(t, string(outContent), "db")

	// Reset flag for other tests?
	// Not strictly necessary as this is likely the only test using this flag,
	// but good practice if tests were parallel.
	_ = cmd.Flags().Set("out", "")
}
