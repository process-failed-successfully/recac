package jira

import (
	"testing"
)

func TestDependencyGraph_GetReadyTickets(t *testing.T) {
	graph := NewDependencyGraph()

	// A -> B (A blocks B)
	graph.AddDependency("A", "B")
	graph.AddTicket("A")
	graph.AddTicket("B")

	completed := make(map[string]bool)

	// Only A should be ready (no blockers)
	ready := graph.GetReadyTickets(completed)
	if len(ready) != 1 || ready[0] != "A" {
		t.Errorf("Expected [A], got %v", ready)
	}

	// Complete A
	completed["A"] = true
	ready = graph.GetReadyTickets(completed)
	if len(ready) != 1 || ready[0] != "B" {
		t.Errorf("Expected [B], got %v", ready)
	}
}

func TestBuildGraphFromIssues(t *testing.T) {
	issues := []map[string]interface{}{
		{
			"key": "A",
			"fields": map[string]interface{}{
				"status": map[string]interface{}{"name": "To Do"},
			},
		},
		{
			"key": "B",
			"fields": map[string]interface{}{
				"status": map[string]interface{}{"name": "Done"},
			},
		},
	}

	// Custom blocker extractor
	// Assume B blocks A
	getBlockers := func(issue map[string]interface{}) []string {
		if issue["key"] == "A" {
			return []string{"B (Done)"} // B blocks A
		}
		return nil
	}

	graph := BuildGraphFromIssues(issues, getBlockers)

	// B -> A
	// B is in graph? Yes.

	completed := make(map[string]bool)
	completed["B"] = true // B is done

	ready := graph.GetReadyTickets(completed)
	// A should be ready because B is done
	if len(ready) != 1 || ready[0] != "A" {
		t.Errorf("Expected [A], got %v", ready)
	}
}

func TestBuildGraphFromIssues_Complex(t *testing.T) {
	// A -> B -> C
	issues := []map[string]interface{}{
		{"key": "A"},
		{"key": "B"},
		{"key": "C"},
	}

	getBlockers := func(issue map[string]interface{}) []string {
		key := issue["key"].(string)
		if key == "B" {
			return []string{"A (Done)"}
		}
		if key == "C" {
			return []string{"B (Done)"}
		}
		return nil
	}

	graph := BuildGraphFromIssues(issues, getBlockers)

	sorted, err := graph.TopologicalSort()
	if err != nil {
		t.Fatalf("Sort failed: %v", err)
	}

	if len(sorted) != 3 {
		t.Errorf("Expected 3 items sorted, got %d", len(sorted))
	}
	// A, B, C
	if sorted[0] != "A" || sorted[1] != "B" || sorted[2] != "C" {
		t.Errorf("Expected [A B C], got %v", sorted)
	}
}

func TestGetReadyTickets_Cycle(t *testing.T) {
	graph := NewDependencyGraph()
	// A -> B -> A
	graph.AddDependency("A", "B")
	graph.AddDependency("B", "A")

	completed := make(map[string]bool)
	ready := graph.GetReadyTickets(completed)

	// Neither is ready because both are blocked
	if len(ready) != 0 {
		t.Errorf("Expected [] for cycle, got %v", ready)
	}
}

func TestDependencyGraph_SelfRef(t *testing.T) {
	issues := []map[string]interface{}{
		{"key": "A"},
	}

	getBlockers := func(issue map[string]interface{}) []string {
		return []string{"A (Status)"} // Self reference
	}

	graph := BuildGraphFromIssues(issues, getBlockers)
	// Self dependency should be ignored by BuildGraphFromIssues

	sorted, _ := graph.TopologicalSort()
	if len(sorted) != 1 || sorted[0] != "A" {
		t.Errorf("Expected [A], got %v", sorted)
	}
}

func TestResolveDependencies_Integration(t *testing.T) {
	// Replicating logic of ResolveDependencies using explicit fetchBlockers
	issues := []map[string]interface{}{
		{"key": "B"},
		{"key": "A"},
	}

	fetchBlockers := func(issue map[string]interface{}) ([]string, error) {
		if issue["key"] == "B" {
			return []string{"A"}, nil
		}
		return nil, nil
	}

	sorted, err := ResolveDependencies(issues, fetchBlockers)
	if err != nil {
		t.Fatalf("Resolve failed: %v", err)
	}

	if len(sorted) != 2 {
		t.Fatalf("Expected 2 items")
	}
	if sorted[0]["key"] != "A" || sorted[1]["key"] != "B" {
		t.Errorf("Expected [A, B], got [%v, %v]", sorted[0]["key"], sorted[1]["key"])
	}
}
