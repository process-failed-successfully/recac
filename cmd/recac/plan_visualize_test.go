package main

import (
	"recac/internal/db"
	"strings"
	"testing"
)

func TestGenerateMermaidPlan(t *testing.T) {
	list := db.FeatureList{
		ProjectName: "Test Project",
		Features: []db.Feature{
			{
				ID:          "F-001",
				Description: "Setup base project",
				Priority:    "MVP",
			},
			{
				ID:          "F-002",
				Description: "Implement login",
				Priority:    "MVP",
				Dependencies: db.FeatureDependencies{
					DependsOnIDs: []string{"F-001"},
				},
			},
			{
				ID:          "F-003",
				Description: "Advanced analytics",
				Priority:    "Production",
				Dependencies: db.FeatureDependencies{
					DependsOnIDs: []string{"F-002"},
				},
			},
			{
				ID:          "F-004",
				Description: "Experimental feature",
				Priority:    "POC",
			},
		},
	}

	mermaid := generateMermaidPlan(list)

	// Assertions
	if !strings.Contains(mermaid, "graph TD") {
		t.Error("Expected graph TD header")
	}

	// Check Nodes
	// Sanitized IDs: F-001 -> F_001
	if !strings.Contains(mermaid, `F_001["F-001: Setup base project"]:::mvp`) {
		t.Errorf("Missing or incorrect node for F-001. Got:\n%s", mermaid)
	}
	if !strings.Contains(mermaid, `F_002["F-002: Implement login"]:::mvp`) {
		t.Errorf("Missing or incorrect node for F-002")
	}
	if !strings.Contains(mermaid, `F_004["F-004: Experimental feature"]:::poc`) {
		t.Errorf("Missing or incorrect node for F-004")
	}

	// Check Edges
	// Logic: F-002 depends on F-001. Code generates: depID --> ID.
	// So F-001 --> F-002.
	if !strings.Contains(mermaid, "F_001 --> F_002") {
		t.Errorf("Missing edge F-001 -> F-002")
	}
	if !strings.Contains(mermaid, "F_002 --> F_003") {
		t.Errorf("Missing edge F-002 -> F-003")
	}

	// Check Styles
	if !strings.Contains(mermaid, "classDef mvp") {
		t.Error("Missing classDef mvp")
	}
}
