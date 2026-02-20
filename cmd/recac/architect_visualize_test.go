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
		{"my-service", "my_service"},
		{"db_1", "db_1"},
		{"User Service", "User_Service"},
		{"special@char!", "special_char_"},
		{"123", "123"},
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
				ID:   "web-ui",
				Type: "frontend",
				Consumes: []architecture.Input{
					{Source: "backend-api", Type: "http"},
				},
			},
			{
				ID:   "backend-api",
				Type: "service",
				Produces: []architecture.Output{
					{Target: "db-main", Type: "sql"},
					{Target: "queue-jobs", Type: "msg"},
				},
			},
			{
				ID:   "db-main",
				Type: "database",
			},
			{
				ID:   "queue-jobs",
				Type: "queue",
			},
			{
				ID:   "worker-1",
				Type: "worker",
				Consumes: []architecture.Input{
					{Source: "queue-jobs", Type: "msg"},
				},
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Check for nodes
	expectedNodes := []string{
		"web_ui(\"web-ui<br/>(frontend)\")",
		"backend_api[\"backend-api<br/>(service)\"]",
		"db_main[(\"db-main<br/>(database)\")]",
		"queue_jobs>\"queue-jobs<br/>(queue)\"]",
		"worker_1{{\"worker-1<br/>(worker)\"}}",
	}

	for _, node := range expectedNodes {
		if !strings.Contains(mermaid, node) {
			t.Errorf("Mermaid output missing node: %s\nGot:\n%s", node, mermaid)
		}
	}

	// Check for edges
	expectedEdges := []string{
		"backend_api -->|http| web_ui", // Wait, Consumes means Source -> Target. So backend-api -> web-ui?
		// Consumes: web-ui consumes from backend-api. So backend-api -> web-ui. Correct.
		"backend_api -->|sql| db_main",
		"backend_api -->|msg| queue_jobs",
		"queue_jobs -->|msg| worker_1",
	}

	for _, edge := range expectedEdges {
		if !strings.Contains(mermaid, edge) {
			t.Errorf("Mermaid output missing edge: %s\nGot:\n%s", edge, mermaid)
		}
	}
}

func TestGenerateHTML(t *testing.T) {
	mermaid := "graph TD; A-->B;"
	html := generateHTML(mermaid)

	if !strings.Contains(html, mermaid) {
		t.Error("HTML output does not contain mermaid graph")
	}
	if !strings.Contains(html, "<script src=\"https://cdn.jsdelivr.net/npm/mermaid/dist/mermaid.min.js\"></script>") {
		t.Error("HTML output missing mermaid script")
	}
}
