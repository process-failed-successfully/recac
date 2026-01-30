package main

import (
	"bytes"
	"os"
	"path/filepath"
	"recac/internal/analysis"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	// We want to verify that the label generated from ID is sanitized in output.

	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"pkg.Func\"Quote": {ID: "pkg.Func\"Quote", Name: "Func\"Quote", Package: "pkg"},
		},
		Edges: nil,
	}

	output := generateMermaidCallGraph(cg)

	// We expect the label to be sanitized.
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
	// Setup temporary directory with sample code for analysis
	tmpDir := t.TempDir()

	mainContent := `package main
func main() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Use factory to create a fresh command instance
	cmd := NewCallGraphCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Set flags
	cmd.SetArgs([]string{"--dir", tmpDir, "--focus", "NonExistentFunctionXYZ"})

	// Run command
	// ExecuteC calls RunE
	err = cmd.Execute()
	assert.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph LR")

	lines := strings.Split(strings.TrimSpace(output), "\n")
	assert.Equal(t, 1, len(lines), "Should only contain graph header for empty result")
	assert.Equal(t, "graph LR", lines[0])
}
