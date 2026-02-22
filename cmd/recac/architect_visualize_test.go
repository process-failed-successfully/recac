package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestArchitectVisualize(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "architect-visualize-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a sample architecture.yaml
	archFile := filepath.Join(tmpDir, "architecture.yaml")
	content := `
version: "1.0"
system_name: "TestSystem"
components:
  - id: "service-a"
    type: "service"
    consumes:
      - source: "queue-1"
        type: "OrderPlaced"
    produces:
      - target: "db-1"
        type: "OrderRecord"

  - id: "db-1"
    type: "database"

  - id: "queue-1"
    type: "queue"
`
	if err := os.WriteFile(archFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	// Case 1: Raw Mermaid Output
	output, err := executeCommand(rootCmd, "architect", "visualize", "--file", archFile)
	assert.NoError(t, err)
	assert.Contains(t, output, "graph TD")
	assert.Contains(t, output, "service_a[service-a]")
	assert.Contains(t, output, "db_1[(db-1)]")
	assert.Contains(t, output, "queue_1{{queue-1}}")
	assert.Contains(t, output, "queue_1 -- OrderPlaced --> service_a")
	assert.Contains(t, output, "service_a -- OrderRecord --> db_1")

	// Case 2: HTML Output
	output, err = executeCommand(rootCmd, "architect", "visualize", "--file", archFile, "--html")
	assert.NoError(t, err)

	htmlFile := filepath.Join(tmpDir, "architecture.html")
	assert.Contains(t, output, fmt.Sprintf("Architecture visualization saved to %s", htmlFile))

	htmlContent, err := os.ReadFile(htmlFile)
	assert.NoError(t, err)
	assert.Contains(t, string(htmlContent), "<!DOCTYPE html>")
	assert.Contains(t, string(htmlContent), "graph TD")
	assert.Contains(t, string(htmlContent), "service_a[service-a]")
}
