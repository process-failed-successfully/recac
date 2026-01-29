package main

import (
	"recac/internal/db"
	"strings"
	"testing"
)

func TestGenerateMermaidPlan(t *testing.T) {
	features := db.FeatureList{
		ProjectName: "Test Project",
		Features: []db.Feature{
			{
				ID:          "F-002",
				Description: "Feature 2",
				Status:      "in_progress",
				Dependencies: db.FeatureDependencies{
					DependsOnIDs: []string{"F-001"},
				},
			},
			{
				ID:          "F-001",
				Description: "Feature 1",
				Status:      "done",
				Passes:      true,
			},
			{
				ID:          "F-003",
				Description: "Feature 3",
				Status:      "pending",
			},
		},
	}

	mermaid := generateMermaidPlan(features)

	// Verify Title
	if !strings.Contains(mermaid, "title Test Project") {
		t.Errorf("Expected title in mermaid, got: %s", mermaid)
	}

	// Verify Node Existence (Sorted F-001, F-002, F-003)
	if !strings.Contains(mermaid, "F_001[\"F-001<br/>Feature 1\"]:::done") {
		t.Errorf("Expected F-001 node with done style, got: %s", mermaid)
	}
	if !strings.Contains(mermaid, "F_002[\"F-002<br/>Feature 2\"]:::active") {
		t.Errorf("Expected F-002 node with active style, got: %s", mermaid)
	}

	// Verify Edge
	if !strings.Contains(mermaid, "F_001 --> F_002") {
		t.Errorf("Expected edge F-001 --> F-002, got: %s", mermaid)
	}

	// Verify Determinism (Order of nodes)
	// F-001 should appear before F-002 in the definition section
	// Note: Edges appear after nodes definition loop
	// We check the order of node definitions
	idx1 := strings.Index(mermaid, "F_001[")
	idx2 := strings.Index(mermaid, "F_002[")
	if idx1 == -1 || idx2 == -1 || idx1 > idx2 {
		t.Errorf("Expected sorted nodes (F-001 before F-002), got indices %d, %d", idx1, idx2)
	}
}

func TestSanitizePlanID(t *testing.T) {
	tests := []struct {
		input  string
		expect string
	}{
		{"F-001", "F_001"},
		{"My Feature", "My_Feature"},
		{"User/Login", "User_Login"},
	}

	for _, tt := range tests {
		if got := sanitizePlanID(tt.input); got != tt.expect {
			t.Errorf("sanitizePlanID(%q) = %q, want %q", tt.input, got, tt.expect)
		}
	}
}
