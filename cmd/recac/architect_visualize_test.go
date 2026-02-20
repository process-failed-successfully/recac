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
		{"User Service", "User_Service"},
		{"db-primary", "db_primary"},
		{"weird@char!", "weird_char_"},
		{"NormalID", "NormalID"},
	}

	for _, tc := range tests {
		got := sanitizeArchID(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeArchID(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestEscapeMermaidLabel(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{`Normal`, `Normal`},
		{`With "Quotes"`, `With #quot;Quotes#quot;`},
		{`Left < Right >`, `Left #lt; Right #gt;`},
		{`A | B`, `A #124; B`},
		{`Me & You`, `Me & You`},
		{`Mix " & < > |`, `Mix #quot; & #lt; #gt; #124;`},
	}

	for _, tc := range tests {
		got := escapeMermaidLabel(tc.input)
		if got != tc.expected {
			t.Errorf("escapeMermaidLabel(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}
}

func TestGenerateMermaidSystemArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{
				ID:   "UserService",
				Type: "service",
				Consumes: []architecture.Input{
					{Source: "Auth|Service", Type: "UserAuth"},
				},
				Produces: []architecture.Output{
					{Target: "UserDB", Type: "User Data"},
				},
			},
			{
				ID:   "UserDB",
				Type: "database",
			},
			{
				ID:   "WorkerNode",
				Type: "worker",
				Consumes: []architecture.Input{
					{Source: "UserDB", Type: "SyncEvent"},
				},
			},
			{
				ID: "Frontend",
				Type: "ui",
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Check for graph definition
	if !strings.Contains(mermaid, "graph TD") {
		t.Error("Mermaid output should contain 'graph TD'")
	}

	// Check for nodes
	expectedNodes := []string{
		`UserService["UserService<br/>(service)"]`,
		`UserDB[("UserDB<br/>(database)")]`,
		`WorkerNode{{"WorkerNode<br/>(worker)"}}`,
		`Frontend("Frontend<br/>(ui)")`,
	}

	for _, node := range expectedNodes {
		if !strings.Contains(mermaid, node) {
			t.Errorf("Mermaid output missing node: %s", node)
		}
	}

	// Check for edges and escaping
	// Auth|Service -> Auth_Service in ID, but label should have #124;
	if !strings.Contains(mermaid, "Auth_Service") {
		t.Error("Mermaid output missing sanitized ID for Auth|Service")
	}
	// "Auth|Service" label in node definition
	if !strings.Contains(mermaid, `Auth_Service["Auth#124;Service"]`) {
		t.Errorf("Mermaid output missing escaped label for Auth|Service source")
	}

	expectedEdges := []string{
		"Auth_Service -->|UserAuth| UserService",
		"UserService -->|User Data| UserDB",
		"UserDB -->|SyncEvent| WorkerNode",
	}

	for _, edge := range expectedEdges {
		if !strings.Contains(mermaid, edge) {
			t.Errorf("Mermaid output missing edge: %s", edge)
		}
	}
}

func TestGenerateHTML(t *testing.T) {
	mermaid := "graph TD\n    A --> B"
	html := generateHTML(mermaid)

	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("HTML output should contain doctype")
	}
	if !strings.Contains(html, "mermaid.initialize") {
		t.Error("HTML output should initialize mermaid")
	}
	// Since we removed template.HTML cast, text/template will escape content.
	// "graph TD" has no special chars, but "-->" contains ">" which becomes "&gt;"
	if !strings.Contains(html, "graph TD") {
		t.Error("HTML output should contain 'graph TD'")
	}
	// A --> B becomes A --&gt; B
	expectedEscaped := "A --&gt; B"
	if !strings.Contains(html, expectedEscaped) {
		t.Errorf("HTML output should contain the escaped mermaid graph. Expected '%s'", expectedEscaped)
	}

	// Test with special chars
	mermaidSpecial := "A -->|Check >| B"
	htmlSpecial := generateHTML(mermaidSpecial)
	// Expect escaping: > becomes &gt;
	expectedSpecial := "A --&gt;|Check &gt;| B"
	if !strings.Contains(htmlSpecial, expectedSpecial) {
		t.Errorf("HTML output should be escaped. Got: %s", htmlSpecial)
	}
}
