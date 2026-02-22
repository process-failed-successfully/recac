package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchitectVisualize(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	archFile := filepath.Join(tmpDir, "architecture.yaml")

	yamlContent := `
version: "1.0"
system_name: "TestSystem"
components:
  - id: "frontend"
    type: "frontend"
    description: "User Interface"
    consumes: []
    produces:
      - target: "backend"
        type: "HTTP"

  - id: "backend"
    type: "service"
    description: "API Service"
    consumes:
      - source: "frontend"
        type: "HTTP"
    produces:
      - target: "db"
        type: "SQL"
      - event: "USER_CREATED"
        target: "queue"

  - id: "db"
    type: "database"
    description: "Main DB"
    consumes:
      - source: "backend"
        type: "SQL"

  - id: "queue"
    type: "queue"
    description: "Message Queue"
    consumes:
      - source: "backend"
        type: "USER_CREATED"
`
	err := os.WriteFile(archFile, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Execute command
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Since runArchitectVisualize expects a *cobra.Command and []string
	// We call it directly
	err = runArchitectVisualize(cmd, []string{tmpDir})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "frontend")
	assert.Contains(t, output, "backend")
	assert.Contains(t, output, "db")
	assert.Contains(t, output, "queue")

	// Check shapes
	assert.Contains(t, output, "((") // frontend circle
	assert.Contains(t, output, "[(") // database cylinder
	assert.Contains(t, output, "{{") // queue hexagon

	// Check edges
	assert.Contains(t, output, "-->|HTTP|")
	assert.Contains(t, output, "-->|SQL|")
	assert.Contains(t, output, "-->|Event: USER_CREATED|")
}

func TestArchitectVisualize_MissingFile(t *testing.T) {
	tmpDir := t.TempDir()
	buf := new(bytes.Buffer)
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(buf)

	err := runArchitectVisualize(cmd, []string{filepath.Join(tmpDir, "missing.yaml")})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to access")
}

func TestArchitectVisualize_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	archFile := filepath.Join(tmpDir, "architecture.yaml")
	err := os.WriteFile(archFile, []byte("invalid: [ yaml"), 0644)
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	cmd := &cobra.Command{Use: "test"}
	cmd.SetOut(buf)

	err = runArchitectVisualize(cmd, []string{archFile})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse YAML")
}
