package main

import (
	"strings"
	"testing"

	"recac/internal/architecture"
)

func TestGenerateMermaidSystemArchitecture(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{
				ID:   "OrderService",
				Type: "service",
				Consumes: []architecture.Input{
					{Source: "UserDB", Type: "UserRecord"},
				},
				Produces: []architecture.Output{
					{Target: "OrderQueue", Type: "OrderPlaced", Event: "OrderPlacedEvent"},
				},
			},
			{
				ID:   "UserDB",
				Type: "database",
			},
			{
				ID:   "OrderQueue",
				Type: "queue",
			},
			{
				ID:   "EmailWorker",
				Type: "worker",
				Consumes: []architecture.Input{
					{Source: "OrderQueue", Type: "OrderPlaced"},
				},
			},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// Check Nodes
	if !strings.Contains(mermaid, `EmailWorker{{"EmailWorker<br/>(worker)"}}`) {
		t.Errorf("Expected EmailWorker node with worker shape, got: %s", mermaid)
	}
	if !strings.Contains(mermaid, `OrderService["OrderService<br/>(service)"]`) {
		t.Errorf("Expected OrderService node with rect shape, got: %s", mermaid)
	}
	if !strings.Contains(mermaid, `UserDB[("UserDB<br/>(database)")]`) {
		t.Errorf("Expected UserDB node with database shape, got: %s", mermaid)
	}
	if !strings.Contains(mermaid, `OrderQueue>"OrderQueue<br/>(queue)"]`) {
		t.Errorf("Expected OrderQueue node with queue shape, got: %s", mermaid)
	}

	// Check Edges
	// UserDB -> OrderService (Consumes)
	if !strings.Contains(mermaid, `UserDB -->|UserRecord| OrderService`) {
		t.Errorf("Expected edge UserDB -> OrderService, got: %s", mermaid)
	}

	// OrderService -> OrderQueue (Produces)
	// Label should include Event info
	if !strings.Contains(mermaid, `OrderService -->|Event: OrderPlacedEvent| OrderQueue`) {
		t.Errorf("Expected edge OrderService -> OrderQueue, got: %s", mermaid)
	}

	// OrderQueue -> EmailWorker (Consumes)
	if !strings.Contains(mermaid, `OrderQueue -->|OrderPlaced| EmailWorker`) {
		t.Errorf("Expected edge OrderQueue -> EmailWorker, got: %s", mermaid)
	}
}

func TestGenerateMermaidSanitization(t *testing.T) {
	arch := &architecture.SystemArchitecture{
		Components: []architecture.Component{
			{ID: "My-Service", Type: "service"},
			{ID: "123DB", Type: "database"},
		},
	}

	mermaid := generateMermaidSystemArchitecture(arch)

	// My-Service -> My_Service
	if !strings.Contains(mermaid, `My_Service["My-Service<br/>(service)"]`) {
		t.Errorf("Expected sanitized My_Service node, got: %s", mermaid)
	}
	// 123DB -> n123DB (cannot start with number)
	if !strings.Contains(mermaid, `n123DB[("123DB<br/>(database)")]`) {
		t.Errorf("Expected sanitized n123DB node, got: %s", mermaid)
	}
}
