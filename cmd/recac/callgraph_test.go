package main

import (
	"recac/internal/analysis"
	"strings"
	"testing"
)

func TestGenerateMermaidCallGraph(t *testing.T) {
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"pkg.Func": {
				ID: "pkg.Func",
				Name: "Func",
				Package: "pkg",
			},
			"pkg.(Type).Method": {
				ID: "pkg.(Type).Method",
				Name: "Method",
				Package: "pkg",
				Receiver: "Type",
			},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "pkg.Func", To: "pkg.(Type).Method"},
		},
	}

	output := generateMermaidCallGraph(cg)

	// Verify output
	if !strings.Contains(output, "graph LR") {
		t.Error("Output should contain graph definition")
	}

	// Check edges
	// Expect: pkg_Func --> pkg__Type__Method
	expectedEdge := "pkg_Func --> pkg__Type__Method"
	if !strings.Contains(output, expectedEdge) {
		t.Errorf("Output missing sanitized edge: %s. Got:\n%s", expectedEdge, output)
	}

	// Check node definitions
	// Expect: pkg__Type__Method["pkg.(Type).Method"]
	expectedNode := "pkg__Type__Method[\"pkg.(Type).Method\"]"
	if !strings.Contains(output, expectedNode) {
		t.Errorf("Output missing sanitized node definition: %s. Got:\n%s", expectedNode, output)
	}
}
