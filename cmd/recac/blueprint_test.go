package main

import (
	"os"
	"path/filepath"
	"testing"

	"recac/internal/architecture"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateMermaidBlueprint(t *testing.T) {
	// Setup mock architecture
	arch := &architecture.SystemArchitecture{
		Version:    "1.0",
		SystemName: "TestSystem",
		Components: []architecture.Component{
			{
				ID:          "api_gateway",
				Type:        "service",
				Description: "Entry point",
				Consumes:    []architecture.Input{},
				Produces: []architecture.Output{
					{Target: "auth_service", Type: "gRPC"},
					{Target: "order_service", Type: "REST"},
				},
			},
			{
				ID:          "auth_service",
				Type:        "service",
				Description: "Handles authentication",
				Consumes: []architecture.Input{
					{Source: "api_gateway", Type: "gRPC"},
				},
				Produces: []architecture.Output{
					{Target: "db", Type: "SQL"},
				},
			},
			{
				ID:          "order_service",
				Type:        "service",
				Description: "Manages orders",
				Consumes: []architecture.Input{
					{Source: "api_gateway", Type: "REST"},
				},
				Produces: []architecture.Output{
					{Target: "db", Type: "SQL"},
					{Target: "event_bus", Type: "Event", Event: "OrderCreated"},
				},
			},
			{
				ID:   "db",
				Type: "database",
			},
			{
				ID:   "event_bus",
				Type: "message_broker",
			},
		},
	}

	// Generate
	mermaid := generateMermaidBlueprint(arch)

	// Verify
	assert.Contains(t, mermaid, "graph TD")

	// Nodes exist
	assert.Contains(t, mermaid, "api_gateway[\"api_gateway<br/>(service)<br/>Entry point\"]")
	assert.Contains(t, mermaid, "auth_service[\"auth_service<br/>(service)<br/>Handles authentication\"]")
	assert.Contains(t, mermaid, "db[\"db<br/>(database)\"]")

	// Edges exist
	// Note: Edges are deduplicated. api_gateway produces to auth_service, and auth_service consumes from api_gateway.
	// This results in one edge: api_gateway -->|gRPC| auth_service
	assert.Contains(t, mermaid, "api_gateway -->|gRPC| auth_service")
	assert.Contains(t, mermaid, "api_gateway -->|REST| order_service")
	assert.Contains(t, mermaid, "auth_service -->|SQL| db")
	assert.Contains(t, mermaid, "order_service -->|SQL| db")
	assert.Contains(t, mermaid, "order_service -->|Event/OrderCreated| event_bus")
}

func TestBlueprintCmd(t *testing.T) {
	// Create temp architecture.yaml
	tmpDir := t.TempDir()
	yamlContent := `
version: "1.0"
system_name: "CMD Test"
components:
  - id: "comp_a"
    type: "service"
    produces:
      - target: "comp_b"
        type: "http"
  - id: "comp_b"
    type: "service"
`
	inputPath := filepath.Join(tmpDir, "architecture.yaml")
	err := os.WriteFile(inputPath, []byte(yamlContent), 0644)
	require.NoError(t, err)

	outputPath := filepath.Join(tmpDir, "output.mmd")

	// Run command via rootCmd to ensure proper context
	rootCmd.SetArgs([]string{"blueprint", "--input", inputPath, "--output", outputPath})

	// Capture stdout
	// We can't easily capture stdout of cobra command execution in this context without more scaffolding,
	// but we can verify the output file creation.
	err = rootCmd.Execute()
	require.NoError(t, err)

	// Verify output file
	outBytes, err := os.ReadFile(outputPath)
	require.NoError(t, err)
	outString := string(outBytes)

	assert.Contains(t, outString, "graph TD")
	assert.Contains(t, outString, "comp_a -->|http| comp_b")
}

func TestSanitizeBlueprintID(t *testing.T) {
	assert.Equal(t, "valid_id", sanitizeBlueprintID("valid_id"))
	assert.Equal(t, "invalid_id", sanitizeBlueprintID("invalid-id"))
	assert.Equal(t, "pkg_struct", sanitizeBlueprintID("pkg.struct"))
	assert.Equal(t, "Space_Name", sanitizeBlueprintID("Space Name"))
}
