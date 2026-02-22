package architecture

import (
	"strings"
	"testing"
)

func TestGenerateMermaid(t *testing.T) {
	arch := &SystemArchitecture{
		Version:    "1.0",
		SystemName: "TestSystem",
		Components: []Component{
			{ID: "Frontend", Type: "frontend"},
			{ID: "Backend", Type: "service", Consumes: []Input{{Source: "Frontend", Type: "HTTP"}}},
			{ID: "Database", Type: "database"},
			{ID: "Queue", Type: "queue"},
			{ID: "Worker", Type: "worker", Consumes: []Input{{Source: "Queue", Type: "Job"}}},
		},
	}

	// Add explicit production link
	arch.Components[1].Produces = []Output{{Target: "Database", Type: "SQL"}}
	arch.Components[1].Produces = append(arch.Components[1].Produces, Output{Target: "Queue", Event: "TaskCreated"})

	mermaid := GenerateMermaid(arch)

	// Check if all nodes are present with correct shapes
	if !strings.Contains(mermaid, "Frontend((\"Frontend\"))") {
		t.Error("Frontend node missing or incorrect shape")
	}
	if !strings.Contains(mermaid, "Backend[\"Backend\"]") {
		t.Error("Backend node missing or incorrect shape")
	}
	if !strings.Contains(mermaid, "Database[(\"Database\")]") {
		t.Error("Database node missing or incorrect shape")
	}
	if !strings.Contains(mermaid, "Queue{{\"Queue\"}}") {
		t.Error("Queue node missing or incorrect shape")
	}
	if !strings.Contains(mermaid, "Worker([\"Worker\"])") {
		t.Error("Worker node missing or incorrect shape")
	}

	// Check connections
	if !strings.Contains(mermaid, "Frontend -->|\"HTTP\"| Backend") {
		t.Error("Frontend -> Backend connection missing")
	}
	if !strings.Contains(mermaid, "Backend -->|\"SQL\"| Database") {
		t.Error("Backend -> Database connection missing")
	}
	if !strings.Contains(mermaid, "Backend -->|\"TaskCreated\"| Queue") {
		t.Error("Backend -> Queue connection missing")
	}
	if !strings.Contains(mermaid, "Queue -->|\"Job\"| Worker") {
		t.Error("Queue -> Worker connection missing")
	}
}

func TestGenerateMermaid_Quotes(t *testing.T) {
	arch := &SystemArchitecture{
		Version:    "1.0",
		SystemName: "QuoteSystem",
		Components: []Component{
			{ID: "My \"Special\" Service", Type: "service"},
		},
	}

	mermaid := GenerateMermaid(arch)

	// ID should be sanitized: My__Special__Service
	// Label should be escaped: My \"Special\" Service
	if !strings.Contains(mermaid, "My__Special__Service[\"My \\\"Special\\\" Service\"]") {
		t.Errorf("Quote escaping failed. Got: %s", mermaid)
	}
}
