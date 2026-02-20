package main

import (
	"testing"

	"recac/internal/architecture"
	"github.com/stretchr/testify/assert"
)

func TestSanitizeArchID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"ServiceA", "ServiceA"},
		{"Service-A", "Service_A"},
		{"Service A", "Service_A"},
		{"db.v1", "db_v1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizeArchID(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestEscapeMermaidLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Normal", "Normal"},
		{"With \"quotes\"", "With #quot;quotes#quot;"},
		{"With <brackets>", "With #lt;brackets#gt;"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeMermaidLabel(tt.input)
			assert.Equal(t, tt.expected, got)
		})
	}
}

func TestGetShape(t *testing.T) {
	tests := []struct {
		compType   string
		wantStart  string
		wantEnd    string
	}{
		{"service", "[", "]"},
		{"database", "[(", ")]"},
		{"db", "[(", ")]"},
		{"worker", "{{", "}}"},
		{"queue", ">", "]"},
		{"frontend", "(", ")"},
		{"ui", "(", ")"},
		{"unknown", "[", "]"},
	}

	for _, tt := range tests {
		t.Run(tt.compType, func(t *testing.T) {
			s, e := getShape(tt.compType)
			assert.Equal(t, tt.wantStart, s)
			assert.Equal(t, tt.wantEnd, e)
		})
	}
}

func TestGenerateMermaidSystemArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{
				ID:          "api-gateway",
				Type:        "service",
				Description: "Entry point",
				Consumes:    nil,
				Produces: []architecture.Output{
					{Target: "auth-service", Event: "Authenticate"},
				},
			},
			{
				ID:          "auth-service",
				Type:        "service",
				Description: "Handles auth",
				Consumes:    nil,
				Produces:    nil,
			},
			{
				ID:          "user-db",
				Type:        "database",
				Description: "Stores users",
				Consumes: []architecture.Input{
					{Source: "auth-service", Type: "Query"},
				},
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Check Nodes
	assert.Contains(t, mermaid, "api_gateway[\"api-gateway<br/>Entry point\"]")
	assert.Contains(t, mermaid, "auth_service[\"auth-service<br/>Handles auth\"]")
	assert.Contains(t, mermaid, "user_db[(\"user-db<br/>Stores users\")]") // Database shape

	// Check Edges
	assert.Contains(t, mermaid, "api_gateway -->|Authenticate| auth_service")
	assert.Contains(t, mermaid, "auth_service -->|Query| user_db")
}

func TestGenerateHTML(t *testing.T) {
	mermaid := "graph TD; A-->B;"
	html := generateHTML(mermaid)

	assert.Contains(t, html, "<!DOCTYPE html>")
	assert.Contains(t, html, "graph TD; A-->B;")
	assert.Contains(t, html, "cdn.jsdelivr.net/npm/mermaid")
}
