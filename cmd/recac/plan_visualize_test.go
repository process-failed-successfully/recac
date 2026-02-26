package main

import (
	"bytes"
	"encoding/json"
	"os"
	"recac/internal/db"
	"strings"
	"testing"
)

func TestGenerateMermaidPlan(t *testing.T) {
	features := []db.Feature{
		{
			ID:          "feat-1",
			Description: "Feature 1",
			Priority:    "MVP",
			Status:      "Done",
		},
		{
			ID:          "feat-2",
			Description: "Feature 2",
			Priority:    "High",
			Status:      "Pending",
			Dependencies: db.FeatureDependencies{
				DependsOnIDs: []string{"feat-1"},
			},
		},
		{
			ID:          "feat-3",
			Description: "Feature 3",
			Priority:    "Low",
			Status:      "Failed",
			Dependencies: db.FeatureDependencies{
				DependsOnIDs: []string{"feat-1", "feat-2"},
			},
		},
	}

	list := db.FeatureList{
		ProjectName: "Test Project",
		Features:    features,
	}

	mermaid := generateMermaidPlan(list)

	// Verify basic structure
	if !strings.Contains(mermaid, "graph TD") {
		t.Error("Expected graph TD")
	}

	// Verify nodes
	if !strings.Contains(mermaid, "feat_1[\"feat-1: Feature 1\"]") {
		t.Error("Missing feat-1 node")
	}
	if !strings.Contains(mermaid, "feat_2[\"feat-2: Feature 2\"]") {
		t.Error("Missing feat-2 node")
	}

	// Verify edges
	if !strings.Contains(mermaid, "feat_1 --> feat_2") {
		t.Error("Missing dependency edge feat-1 -> feat-2")
	}
	if !strings.Contains(mermaid, "feat_1 --> feat_3") {
		t.Error("Missing dependency edge feat-1 -> feat-3")
	}
	if !strings.Contains(mermaid, "feat_2 --> feat_3") {
		t.Error("Missing dependency edge feat-2 -> feat-3")
	}

	// Verify styles
	// feat-1 is Done -> green
	if !strings.Contains(mermaid, "style feat_1 fill:#9f9") {
		t.Error("Incorrect style for Done feature")
	}
	// feat-2 is Pending, High -> dashed (High/POC logic) or MVP color?
	// Priority "High" matches "high" -> dashed
	if !strings.Contains(mermaid, "style feat_2 fill:#f9f") {
		t.Error("Incorrect style for High priority feature")
	}
	// feat-3 is Failed -> red
	if !strings.Contains(mermaid, "style feat_3 fill:#f99") {
		t.Error("Incorrect style for Failed feature")
	}
}

func TestPlanVisualizeCmd(t *testing.T) {
	// Create a temp file with JSON content
	features := []db.Feature{
		{ID: "A", Description: "A", Status: "Done"},
		{ID: "B", Description: "B", Status: "Pending", Dependencies: db.FeatureDependencies{DependsOnIDs: []string{"A"}}},
	}
	list := db.FeatureList{Features: features}
	data, _ := json.Marshal(list)

	tmpFile, err := os.CreateTemp("", "plan_test_*.json")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Use isolated command structure
	testPlanCmd := NewPlanCmd()
	testPlanCmd.AddCommand(planVisualizeCmd)

	// Reset planVisualizeCmd parent to ensure isolation from global planCmd/rootCmd if necessary
	// But simply adding it to a new parent should work if we execute the new parent.

	// Capture output
	buf := new(bytes.Buffer)
	testPlanCmd.SetOut(buf)
	testPlanCmd.SetArgs([]string{"visualize", tmpFile.Name()})

	// Execute
	if err := testPlanCmd.Execute(); err != nil {
		t.Fatalf("Execute failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "graph TD") {
		t.Errorf("Expected mermaid output, got: %s", output)
	}
	if !strings.Contains(output, "A --> B") {
		t.Errorf("Expected dependency A -> B, got: %s", output)
	}
}
