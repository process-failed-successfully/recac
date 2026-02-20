package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/architecture"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestSanitizeArchID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"My Service", "My_Service"},
		{"service-1", "service_1"},
		{"db.main", "db_main"},
		{"simple", "simple"},
		{"MixedCASE", "MixedCASE"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := sanitizeArchID(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestEscapeMermaidLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal", "Normal"},
		{"With \"Quote\"", "With #quot;Quote#quot;"},
		{"Generic<T>", "Generic#lt;T#gt;"},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			got := escapeMermaidLabel(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

func TestGenerateMermaidSystemArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{
				ID:   "API Service",
				Type: "service",
				Consumes: []architecture.Input{
					{Source: "User Queue", Type: "Job"},
				},
				Produces: []architecture.Output{
					{Target: "Main DB", Type: "UserRecord"},
				},
			},
			{
				ID:   "Main DB",
				Type: "database",
			},
			{
				ID:   "User Queue",
				Type: "queue",
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Check nodes
	assert.Contains(t, mermaid, "API_Service[\"API Service\"]")
	assert.Contains(t, mermaid, "Main_DB[(\"Main DB\")]")
	assert.Contains(t, mermaid, "User_Queue>\"User Queue\"]")

	// Check edges
	assert.Contains(t, mermaid, "User_Queue -- \"Job\" --> API_Service")
	assert.Contains(t, mermaid, "API_Service -- \"UserRecord\" --> Main_DB")
}

func TestGenerateHTML(t *testing.T) {
	mermaid := "graph TD\n    A --> B"
	html, err := generateHTML(mermaid)
	require.NoError(t, err)
	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "mermaid.initialize")
	assert.Contains(t, html, "graph TD")
	assert.Contains(t, html, "A --> B")
}

func TestRunArchitectVisualize_HTML(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	archFile := filepath.Join(tmpDir, "architecture.yaml")

	arch := architecture.SystemArchitecture{
		SystemName: "TestSystem",
		Components: []architecture.Component{
			{ID: "A", Type: "service"},
			{ID: "B", Type: "db"},
		},
	}
	data, _ := yaml.Marshal(arch)
	require.NoError(t, os.WriteFile(archFile, data, 0644))

	// Prepare command
	cmd := architectVisualizeCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)

	// Reset flags
	cmd.Flags().Set("dir", tmpDir)
	cmd.Flags().Set("html", "true")

	// Execute
	err := runArchitectVisualize(cmd, []string{})
	require.NoError(t, err)

	// Verify HTML file created
	htmlPath := filepath.Join(tmpDir, "architecture.html")
	_, err = os.Stat(htmlPath)
	assert.NoError(t, err)

	// Verify output message
	assert.Contains(t, buf.String(), "Visualization generated at")
}

func TestRunArchitectVisualize_Mermaid(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	archFile := filepath.Join(tmpDir, "architecture.yaml")

	arch := architecture.SystemArchitecture{
		SystemName: "TestSystem",
		Components: []architecture.Component{
			{ID: "A", Type: "service"},
		},
	}
	data, _ := yaml.Marshal(arch)
	require.NoError(t, os.WriteFile(archFile, data, 0644))

	// Prepare command
	cmd := architectVisualizeCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Reset flags
	cmd.Flags().Set("dir", tmpDir)
	cmd.Flags().Set("html", "false")

	// Execute
	err := runArchitectVisualize(cmd, []string{})
	require.NoError(t, err)

	// Verify mermaid output
	assert.Contains(t, buf.String(), "graph TD")
	assert.Contains(t, buf.String(), "A[\"A\"]")
}
