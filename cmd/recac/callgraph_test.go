package main

import (
	"recac/internal/analysis"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFilterGraph(t *testing.T) {
	// Setup Graph
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"pkg.FuncA": {ID: "pkg.FuncA", Name: "FuncA"},
			"pkg.FuncB": {ID: "pkg.FuncB", Name: "FuncB"},
			"pkg.FuncC": {ID: "pkg.FuncC", Name: "FuncC"},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "pkg.FuncA", To: "pkg.FuncB"},
			{From: "pkg.FuncB", To: "pkg.FuncC"},
		},
	}

	// 1. Filter matches FuncB (should include A->B and B->C)
	filtered := filterGraph(cg, "FuncB")
	assert.Len(t, filtered.Nodes, 3) // A, B, C
	assert.Len(t, filtered.Edges, 2)

	// 2. Filter matches FuncA (should include A->B)
	filteredA := filterGraph(cg, "FuncA")
	assert.Len(t, filteredA.Nodes, 2) // A, B
	assert.Len(t, filteredA.Edges, 1)
	assert.Equal(t, "pkg.FuncA", filteredA.Edges[0].From)
	assert.Equal(t, "pkg.FuncB", filteredA.Edges[0].To)

	// 3. No match
	filteredNone := filterGraph(cg, "xyz")
	// Implementation returns original if empty match?
	// Let's check code:
	// if len(relevantNodes) == 0 { return cg }
	assert.Equal(t, cg, filteredNone)
}

func TestGenerateMermaidCallGraph(t *testing.T) {
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"pkg.FuncA": {ID: "pkg.FuncA", Name: "FuncA"},
			"pkg.FuncB": {ID: "pkg.FuncB", Name: "FuncB"},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "pkg.FuncA", To: "pkg.FuncB"},
		},
	}

	out := generateMermaidCallGraph(cg)

	assert.Contains(t, out, "graph LR")
	assert.Contains(t, out, "pkg_FuncA[\"pkg.FuncA\"]")
	assert.Contains(t, out, "pkg_FuncB[\"pkg.FuncB\"]")
	assert.Contains(t, out, "pkg_FuncA --> pkg_FuncB")
}
