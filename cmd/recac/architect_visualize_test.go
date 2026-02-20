package main

import (
	"strings"
	"testing"

	"recac/internal/architecture"
)

func TestSanitizeArchID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"simple", "simple"},
		{"with-dash", "with_dash"},
		{"with space", "with_space"},
		{"mixed-space and dash", "mixed_space_and_dash"},
	}

	for _, tt := range tests {
		got := sanitizeArchID(tt.input)
		if got != tt.expected {
			t.Errorf("sanitizeArchID(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestGenerateMermaidSystemArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{
				ID:   "web-server",
				Type: "service",
				Consumes: []architecture.Input{
					{Source: "user-queue", Type: "UserSignup"},
				},
				Produces: []architecture.Output{
					{Target: "main-db", Type: "UserRecord"},
				},
			},
			{
				ID:   "main-db",
				Type: "database",
			},
			{
				ID:   "user-queue",
				Type: "queue",
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Check for nodes
	if !strings.Contains(mermaid, "web_server[\"web-server<br/>(service)\"]") {
		t.Errorf("Mermaid output missing web-server node. Got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "main_db[(\"main-db<br/>(database)\")]") {
		t.Errorf("Mermaid output missing main-db node. Got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "user_queue>\"user-queue<br/>(queue)\"]") {
		t.Errorf("Mermaid output missing user-queue node. Got:\n%s", mermaid)
	}

	// Check for edges
	if !strings.Contains(mermaid, "user_queue -->|UserSignup| web_server") {
		t.Errorf("Mermaid output missing input edge. Got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "web_server -->|UserRecord| main_db") {
		t.Errorf("Mermaid output missing output edge. Got:\n%s", mermaid)
	}
}

func TestGenerateHTML(t *testing.T) {
	mermaid := "graph TD\n    A --> B"
	html := generateHTML(mermaid)

	if !strings.Contains(html, "<div class=\"mermaid\">") {
		t.Error("HTML missing mermaid div")
	}
	if !strings.Contains(html, mermaid) {
		t.Error("HTML missing mermaid content")
	}
	if !strings.Contains(html, "https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs") {
		t.Error("HTML missing mermaid script")
	}
}
