package main

import (
	"os"
	"path/filepath"
	"testing"

	"recac/internal/architecture"

	"github.com/stretchr/testify/assert"
)

func TestGenerateMermaidSystemArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		SystemName: "TestSys",
		Components: []architecture.Component{
			{
				ID:   "API Gateway",
				Type: "service",
				Consumes: []architecture.Input{
					{Source: "User", Type: "http"},
				},
				Produces: []architecture.Output{
					{Target: "Auth Service", Type: "grpc"},
				},
			},
			{
				ID:   "Auth Service",
				Type: "service",
				Produces: []architecture.Output{
					{Target: "User DB", Type: "sql"},
				},
			},
			{
				ID:   "User DB",
				Type: "database",
			},
			{
				ID:   "Worker",
				Type: "worker",
				Consumes: []architecture.Input{
					{Source: "Queue", Type: "amqp"},
				},
			},
			{
				ID:   "Queue",
				Type: "queue",
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Check Nodes
	assert.Contains(t, mermaid, "API_Gateway[\"API Gateway\"]")
	assert.Contains(t, mermaid, "Auth_Service[\"Auth Service\"]")
	assert.Contains(t, mermaid, "User_DB[(\"User DB\")]")
	assert.Contains(t, mermaid, "Worker{{\"Worker\"}}")
	assert.Contains(t, mermaid, "Queue>\"Queue\"]")

	// Check Edges
	assert.Contains(t, mermaid, "User -->|http| API_Gateway")
	assert.Contains(t, mermaid, "API_Gateway -->|grpc| Auth_Service")
	assert.Contains(t, mermaid, "Auth_Service -->|sql| User_DB")
	assert.Contains(t, mermaid, "Queue -->|amqp| Worker")

	// Check External Node
	assert.Contains(t, mermaid, "User[\"User\"]")
	assert.Contains(t, mermaid, "style User")
}

func TestSanitizeArchID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Hello World", "Hello_World"},
		{"My-Service", "My_Service"},
		{"Valid123", "Valid123"},
		{"a+b=c", "a_b_c"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, sanitizeArchID(tt.input))
	}
}

func TestEscapeMermaidLabel(t *testing.T) {
	assert.Equal(t, "This is 'quoted'", escapeMermaidLabel("This is \"quoted\""))
}

func TestGenerateHTML(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "arch_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "test.html")
	err = writeHTML(path, "Test Architecture", "graph TD\nA-->B")
	assert.NoError(t, err)

	content, err := os.ReadFile(path)
	assert.NoError(t, err)
	html := string(content)

	assert.Contains(t, html, "<title>Test Architecture - Architecture</title>")
	assert.Contains(t, html, "graph TD\nA-->B")
	assert.Contains(t, html, "https://cdn.jsdelivr.net/npm/mermaid@10/dist/mermaid.esm.min.mjs")
}
