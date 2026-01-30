package main

import (
	"bytes"
	"recac/internal/analysis"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCallGraphCmd_Focus_NoMatch(t *testing.T) {
	// Setup a dummy graph
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"pkg.FuncA": {ID: "pkg.FuncA", Name: "FuncA", Package: "pkg"},
			"pkg.FuncB": {ID: "pkg.FuncB", Name: "FuncB", Package: "pkg"},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "pkg.FuncA", To: "pkg.FuncB"},
		},
	}

	// Filter with non-existent focus
	filtered := filterGraph(cg, "NonExistent")

	// Expect empty graph
	assert.Empty(t, filtered.Nodes, "Nodes should be empty when focus matches nothing")
	assert.Empty(t, filtered.Edges, "Edges should be empty when focus matches nothing")
}

func TestCallGraphCmd_Mermaid_Sanitization(t *testing.T) {
	// Setup a dummy graph with quotes in ID (simulating complex type)
	// Note: ID itself is sanitized by sanitizeMermaidID, but we are testing the Label quoting.
	// We want to ensure that if the label (derived from ID) contains quotes, they are escaped/replaced.

	// Assume ID "pkg.(TypeWith\"Quote).Method" -> Label "TypeWith\"Quote).Method"
	// This is unlikely in Go, but defensive coding is good.
	// More likely: user has weird file names or something.

	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"pkg.FuncA": {ID: "pkg.FuncA", Name: "FuncA", Package: "pkg"},
		},
		Edges: nil,
	}

	// We manually inject a node with quote in ID to test label generation
	// But generateMermaidCallGraph takes *analysis.CallGraph.
	// We can't easily inject a node with quote because GenerateCallGraph creates them.
	// But we can construct the CallGraph manually as above.

	// Let's try to verify generateMermaidCallGraph directly.
	// But it's in main package, so we can call it.

	// Override one node to have a label with quotes
	// The label is derived from ID.
	cg.Nodes["pkg.Func\"Quote"] = &analysis.CallGraphNode{
		ID: "pkg.Func\"Quote",
		Name: "Func\"Quote",
		Package: "pkg",
	}

	output := generateMermaidCallGraph(cg)

	// We expect the label to be sanitized.
	// Current implementation: ["Func"Quote"] -> Syntax Error
	// Expected: ["Func'Quote"] or similar.

	// We check if the output contains invalid syntax
	assert.NotContains(t, output, "[\"Func\"Quote\"]", "Should not contain double quotes inside label quotes")
	assert.Contains(t, output, "Func'Quote", "Should replace double quote with single quote")
}

func TestFilterGraph_PartialMatch(t *testing.T) {
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"pkg.FuncA": {ID: "pkg.FuncA", Name: "FuncA", Package: "pkg"},
			"pkg.FuncB": {ID: "pkg.FuncB", Name: "FuncB", Package: "pkg"},
			"pkg.FuncC": {ID: "pkg.FuncC", Name: "FuncC", Package: "pkg"},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "pkg.FuncA", To: "pkg.FuncB"},
			{From: "pkg.FuncB", To: "pkg.FuncC"},
		},
	}

	// Focus on FuncB. Should include FuncA (caller) and FuncC (callee)
	filtered := filterGraph(cg, "FuncB")

	assert.Contains(t, filtered.Nodes, "pkg.FuncA")
	assert.Contains(t, filtered.Nodes, "pkg.FuncB")
	assert.Contains(t, filtered.Nodes, "pkg.FuncC")

	// Edges
	assert.Len(t, filtered.Edges, 2)
}

func TestRunCallGraph_Integration(t *testing.T) {
	// Minimal integration test invoking the command function
	// We need to set flags.
	// Since flags are global variables in callgraph.go, we must be careful with parallel tests.
	// This test is not parallel.

	oldDir := callGraphDir
	oldFocus := callGraphFocus
	defer func() {
		callGraphDir = oldDir
		callGraphFocus = oldFocus
	}()

	callGraphDir = "."
	callGraphFocus = "NonExistentFunctionXYZ"

	cmd := callGraphCmd
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// We run against current directory (recac root).
	// It should parse files.
	// Focus is set to something that doesn't exist.
	// Should return empty graph.

	err := runCallGraph(cmd, []string{})
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph LR")
	// Should NOT contain any nodes except maybe style?
	// If empty, it just has header.
	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 1, len(lines), "Should only contain graph header for empty result")
	assert.Equal(t, "graph LR", lines[0])
}
