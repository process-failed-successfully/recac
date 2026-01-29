package main

import (
	"recac/internal/analysis"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterGraph(t *testing.T) {
	// Setup a sample graph
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"pkg.A": {ID: "pkg.A", Name: "A"},
			"pkg.B": {ID: "pkg.B", Name: "B"},
			"pkg.C": {ID: "pkg.C", Name: "C"},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "pkg.A", To: "pkg.B"},
			{From: "pkg.B", To: "pkg.C"},
		},
	}

	// Test 1: Focus "A" (should include A and B)
	filtered := filterGraph(cg, "A")
	assert.Contains(t, filtered.Nodes, "pkg.A")
	assert.Contains(t, filtered.Nodes, "pkg.B")
	assert.NotContains(t, filtered.Nodes, "pkg.C")
	assert.Len(t, filtered.Edges, 1)
	assert.Equal(t, "pkg.A", filtered.Edges[0].From)
	assert.Equal(t, "pkg.B", filtered.Edges[0].To)

	// Test 2: Focus "C" (should include B and C)
	filtered = filterGraph(cg, "C")
	assert.Contains(t, filtered.Nodes, "pkg.C")
	assert.Contains(t, filtered.Nodes, "pkg.B") // Caller of C
	assert.NotContains(t, filtered.Nodes, "pkg.A")
	assert.Len(t, filtered.Edges, 1)
	assert.Equal(t, "pkg.B", filtered.Edges[0].From)
	assert.Equal(t, "pkg.C", filtered.Edges[0].To)

	// Test 3: Focus "X" (Non-existent)
	filtered = filterGraph(cg, "X")
	assert.Empty(t, filtered.Nodes)
	assert.Empty(t, filtered.Edges)
}

func TestSanitizeMermaidID_Usage(t *testing.T) {
	// Verify SanitizeMermaidID behavior just in case
	id := "pkg/sub.Func"
	safe := SanitizeMermaidID(id)
	assert.Equal(t, "pkg_sub_Func", safe)
}
