package main

import (
	"recac/internal/architecture"
	"strings"
	"testing"
)

func TestSanitizeArchID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-service", "my_service"},
		{"my service", "my_service"},
		{"group/service", "group_service"},
		{"v1.2", "v1_2"},
	}

	for _, tt := range tests {
		got := sanitizeArchID(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeArchID(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestEscapeMermaidLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Price", "Price"},
		{"<br/>", "#lt;br/#gt;"},
		{"\"quoted\"", "#quot;quoted#quot;"},
	}

	for _, tt := range tests {
		got := escapeMermaidLabel(tt.input)
		if got != tt.expected {
			t.Errorf("escapeMermaidLabel(%q) = %q; want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGenerateMermaidSystemArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{
				ID:   "web-ui",
				Type: "frontend",
				Consumes: []architecture.Input{
					{Source: "api-gateway", Type: "HTTP"},
				},
			},
			{
				ID:   "api-gateway",
				Type: "service",
				Produces: []architecture.Output{
					{Target: "auth-service", Type: "gRPC"},
				},
			},
			{
				ID:   "auth-service",
				Type: "service",
			},
			{
				ID:   "users-db",
				Type: "database",
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Check for nodes
	if !strings.Contains(mermaid, "web_ui(") {
		t.Error("Mermaid missing web-ui node or wrong shape")
	}
	if !strings.Contains(mermaid, "api_gateway[") {
		t.Error("Mermaid missing api-gateway node or wrong shape")
	}
	if !strings.Contains(mermaid, "users_db[(") {
		t.Error("Mermaid missing users-db node or wrong shape")
	}

	// Check for edges
	if !strings.Contains(mermaid, "api_gateway -->|HTTP| web_ui") {
		t.Error("Mermaid missing edge api_gateway -> web_ui")
	}
	if !strings.Contains(mermaid, "api_gateway -->|gRPC| auth_service") {
		t.Error("Mermaid missing edge api_gateway -> auth_service")
	}
}

func TestGenerateHTML(t *testing.T) {
	mermaid := "graph TD; A-->B;"
	html := generateHTML(mermaid)

	if !strings.Contains(html, mermaid) {
		t.Error("HTML does not contain mermaid string")
	}
	if !strings.Contains(html, "mermaid.initialize") {
		t.Error("HTML missing mermaid initialization")
	}
}
