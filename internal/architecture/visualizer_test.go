package architecture

import (
	"strings"
	"testing"
)

func TestGenerateMermaid(t *testing.T) {
	arch := &SystemArchitecture{
		SystemName: "TestSystem",
		Components: []Component{
			{
				ID:   "api-gateway",
				Type: "service",
				Consumes: []Input{
					{Source: "auth-service", Type: "grpc"},
				},
			},
			{
				ID:   "auth-service",
				Type: "service",
				Produces: []Output{
					{Target: "user-db", Type: "sql"},
				},
			},
			{
				ID:   "user-db",
				Type: "database",
			},
			{
				ID:   "worker-node",
				Type: "worker",
				Consumes: []Input{
					{Source: "api-gateway", Type: "http"},
				},
			},
		},
	}

	mermaid := GenerateMermaid(arch)

	// Check header
	if !strings.Contains(mermaid, "graph TD") {
		t.Error("Expected graph TD")
	}
	if !strings.Contains(mermaid, "subgraph System [\"TestSystem\"]") {
		t.Error("Expected subgraph System [\"TestSystem\"]")
	}

	// Check Nodes
	if !strings.Contains(mermaid, "api-gateway[\"api-gateway<br/>(service)\"]") {
		t.Errorf("Expected api-gateway node, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "user-db[(\"user-db<br/>(database)\")]") {
		t.Errorf("Expected user-db database node, got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, "worker-node((\"worker-node<br/>(worker)\"))") {
		t.Errorf("Expected worker-node worker node, got:\n%s", mermaid)
	}

	// Check Edges
	// auth-service -> api-gateway (Consumes)
	if !strings.Contains(mermaid, "auth-service -->|grpc| api-gateway") {
		t.Errorf("Expected edge auth-service -->|grpc| api-gateway, got:\n%s", mermaid)
	}
	// auth-service -> user-db (Produces)
	if !strings.Contains(mermaid, "auth-service -->|sql| user-db") {
		t.Errorf("Expected edge auth-service -->|sql| user-db, got:\n%s", mermaid)
	}
	// api-gateway -> worker-node (Consumes)
	if !strings.Contains(mermaid, "api-gateway -->|http| worker-node") {
		t.Errorf("Expected edge api-gateway -->|http| worker-node, got:\n%s", mermaid)
	}
}

func TestGenerateMermaid_Empty(t *testing.T) {
	arch := &SystemArchitecture{
		SystemName: "",
		Components: []Component{},
	}
	mermaid := GenerateMermaid(arch)
	if !strings.Contains(mermaid, "subgraph System [\"System\"]") {
		t.Error("Expected default system name")
	}
}
