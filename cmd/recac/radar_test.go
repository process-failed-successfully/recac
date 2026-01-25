package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRadarCmd(t *testing.T) {
	// Setup Temp Dir
	tmpDir := t.TempDir()

	// Create dummy go.mod to ensure we have "technologies"
	// analyzeStack looks for go.mod
	goModContent := `module example.com/test
go 1.21
require (
	github.com/spf13/cobra v1.8.0
	github.com/gin-gonic/gin v1.9.1
)`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goModContent), 0644))

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	mockResponse := `[
  { "name": "Go", "quadrant": "Languages & Frameworks", "ring": "Adopt", "description": "Core language" },
  { "name": "Cobra", "quadrant": "Languages & Frameworks", "ring": "Adopt", "description": "CLI Lib" },
  { "name": "Gin", "quadrant": "Languages & Frameworks", "ring": "Assess", "description": "Web Framework" }
]`
	mockAgent.SetResponse(mockResponse)

	// Override Factory
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, workspace, projectID string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Execute JSON
	output, err := executeCommand(rootCmd, "radar", tmpDir, "--json")
	require.NoError(t, err)

	// Verify Output
	assert.Contains(t, output, "Cobra")
	assert.Contains(t, output, "Gin")
	assert.Contains(t, output, "Adopt")

	// Execute HTML
	outHtmlPath := filepath.Join(tmpDir, "radar.html")
	outputHTML, err := executeCommand(rootCmd, "radar", tmpDir, "--html", "--out", outHtmlPath)
	require.NoError(t, err)
	assert.Contains(t, outputHTML, "Radar HTML generated")
	assert.FileExists(t, outHtmlPath)
}
