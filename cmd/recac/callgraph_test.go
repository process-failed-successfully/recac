package main

import (
	"testing"
    "recac/internal/analysis"

	"github.com/stretchr/testify/assert"
)

func TestFilterGraph(t *testing.T) {
    // Manually construct a graph
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

    t.Run("Focus A", func(t *testing.T) {
        // Should include A (focus) and B (callee of A)
        filtered := filterGraph(cg, "A")
        assert.Contains(t, filtered.Nodes, "pkg.A")
        assert.Contains(t, filtered.Nodes, "pkg.B")

        _, hasC := filtered.Nodes["pkg.C"]
        assert.False(t, hasC, "C should not be included")
    })

    t.Run("Focus B", func(t *testing.T) {
        // Should include B (focus), A (caller of B), C (callee of B)
        filtered := filterGraph(cg, "B")
        assert.Contains(t, filtered.Nodes, "pkg.B")
        assert.Contains(t, filtered.Nodes, "pkg.A")
        assert.Contains(t, filtered.Nodes, "pkg.C")
    })

     t.Run("Focus NonExistent", func(t *testing.T) {
        filtered := filterGraph(cg, "X")
        assert.Equal(t, len(cg.Nodes), len(filtered.Nodes))
    })
}

func TestGenerateMermaidCallGraph(t *testing.T) {
     cg := &analysis.CallGraph{
        Nodes: map[string]*analysis.CallGraphNode{
            "pkg.A": {ID: "pkg.A", Name: "A"},
            "pkg.B": {ID: "pkg.B", Name: "B"},
        },
        Edges: []analysis.CallGraphEdge{
            {From: "pkg.A", To: "pkg.B"},
        },
    }

    mermaid := generateMermaidCallGraph(cg)
    assert.Contains(t, mermaid, "graph LR")
    // Note: implementation uses sanitized IDs, but for simple "pkg.A", it should be similar or contain the label.
    // implementation: label = parts[len(parts)-1] -> "A", "B"
    assert.Contains(t, mermaid, "A")
    assert.Contains(t, mermaid, "B")
    assert.Contains(t, mermaid, "-->")
}
