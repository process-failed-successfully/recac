package main

import (
	"testing"

	"recac/internal/architecture"

	"github.com/stretchr/testify/assert"
)

func TestGenerateMermaidArch(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		SystemName: "TestSystem",
		Components: []architecture.Component{
			{
				ID:   "frontend",
				Type: "frontend",
				Consumes: []architecture.Input{
					{Source: "api-gateway", Type: "http"},
				},
			},
			{
				ID:   "api-gateway",
				Type: "service",
				Produces: []architecture.Output{
					{Target: "worker", Type: "job", Event: "job_created"},
				},
			},
			{
				ID:   "worker",
				Type: "worker",
				Consumes: []architecture.Input{
					{Source: "db", Type: "query"},
				},
			},
			{
				ID:   "db",
				Type: "database",
			},
		},
	}

	mermaid := generateMermaidArch(arch)

	// Check for graph definition
	assert.Contains(t, mermaid, "graph TD")

	// Check for nodes with correct shapes
	assert.Contains(t, mermaid, "frontend((\"frontend") // frontend uses (( ))
	assert.Contains(t, mermaid, "api_gateway[\"api-gateway") // service uses [ ]
	assert.Contains(t, mermaid, "worker([\"worker") // worker uses ([ ])
	assert.Contains(t, mermaid, "db[(\"db") // database uses [( )]

	// Check for edges (Consumes)
	// frontend consumes from api-gateway -> api-gateway --> frontend
	// Note: sanitizeMermaidID replaces - with _
	assert.Contains(t, mermaid, "api_gateway -- \"http\" --> frontend")

	// Check for edges (Produces)
	// api-gateway produces to worker -> api-gateway --> worker
	assert.Contains(t, mermaid, "api_gateway -- \"job_created\" --> worker")

	// worker consumes from db -> db --> worker
	assert.Contains(t, mermaid, "db -- \"query\" --> worker")
}

func TestSanitizeMermaidID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"with-dash", "with_dash"},
		{"with space", "with_space"},
		{"with.dot", "with_dot"},
	}

	for _, tt := range tests {
		result := sanitizeMermaidID(tt.input)
		assert.Equal(t, tt.expected, result)
	}
}
