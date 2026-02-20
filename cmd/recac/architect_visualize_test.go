package main

import (
	"strings"
	"testing"

	"recac/internal/architecture"
)

func TestGenerateMermaidSystemArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		SystemName: "TestSystem",
		Components: []architecture.Component{
			{
				ID:   "API",
				Type: "service",
				Consumes: []architecture.Input{
					{Source: "User", Type: "HTTP Request"},
				},
				Produces: []architecture.Output{
					{Target: "DB", Type: "SQL"},
				},
			},
			{
				ID:   "DB",
				Type: "database",
			},
			{
				ID:   "Worker",
				Type: "worker",
				Consumes: []architecture.Input{
					{Source: "API", Type: "Job"},
				},
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Verify Nodes
	if !strings.Contains(mermaid, "API[") {
		t.Errorf("Expected API node, got: %s", mermaid)
	}
	// Check for database shape opening "[("
	if !strings.Contains(mermaid, "DB[(") {
		t.Errorf("Expected DB database node, got: %s", mermaid)
	}
	// Check for worker shape opening "("
	if !strings.Contains(mermaid, "Worker(") {
		t.Errorf("Expected Worker node, got: %s", mermaid)
	}

	// Verify Edges
	// API consumes HTTP from User -> User --> API
	if !strings.Contains(mermaid, "User -- \"HTTP Request\" --> API") {
		t.Errorf("Expected User -> API edge, got: %s", mermaid)
	}

	// API produces SQL to DB -> API --> DB
	if !strings.Contains(mermaid, "API -- \"SQL\" --> DB") {
		t.Errorf("Expected API -> DB edge, got: %s", mermaid)
	}

	// Worker consumes Job from API -> API --> Worker
	if !strings.Contains(mermaid, "API -- \"Job\" --> Worker") {
		t.Errorf("Expected API -> Worker edge, got: %s", mermaid)
	}
}

func TestSanitizeArchID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Simple", "Simple"},
		{"With Space", "With_Space"},
		{"With-Dash", "With_Dash"},
		{"With.Dot", "With_Dot"},
		{"Complex/ID:123", "Complex_ID_123"},
	}

	for _, tt := range tests {
		result := sanitizeArchID(tt.input)
		if result != tt.expected {
			t.Errorf("sanitizeArchID(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
