package main

import (
	"os"
	"path/filepath"
	"recac/internal/analysis"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilterGraph(t *testing.T) {
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"A": {ID: "A", Name: "FuncA"},
			"B": {ID: "B", Name: "FuncB"},
			"C": {ID: "C", Name: "FuncC"},
			"D": {ID: "D", Name: "FuncD"},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "A", To: "B"},
			{From: "B", To: "C"},
			{From: "C", To: "D"},
		},
	}

	t.Run("Filter by ID", func(t *testing.T) {
		filtered := filterGraph(cg, "B")
		assert.Contains(t, filtered.Nodes, "A") // Caller
		assert.Contains(t, filtered.Nodes, "B") // Focus
		assert.Contains(t, filtered.Nodes, "C") // Callee
		assert.NotContains(t, filtered.Nodes, "D") // Too far
	})

	t.Run("Filter by Name", func(t *testing.T) {
		filtered := filterGraph(cg, "FuncC")
		assert.Contains(t, filtered.Nodes, "B") // Caller
		assert.Contains(t, filtered.Nodes, "C") // Focus
		assert.Contains(t, filtered.Nodes, "D") // Callee
		assert.NotContains(t, filtered.Nodes, "A") // Too far
	})

	t.Run("Filter No Match", func(t *testing.T) {
		filtered := filterGraph(cg, "Z")
		assert.Equal(t, cg, filtered) // Returns original on no match based on implementation
	})
}

func TestCallGraphCmd(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple Go file
	code := `package main
	func main() {
		foo()
	}
	func foo() {}
	`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(code), 0644)
	require.NoError(t, err)

	root := &cobra.Command{Use: "recac"}
	root.AddCommand(callGraphCmd)

	// Reset flags
	callGraphCmd.Flags().Set("dir", tmpDir)
	callGraphCmd.Flags().Set("focus", "")

	output, err := executeCommand(root, "callgraph", "--dir", tmpDir)
	assert.NoError(t, err)
	assert.Contains(t, output, "graph LR")
	assert.Contains(t, output, "main.main")
	assert.Contains(t, output, "main.foo")
	assert.Contains(t, output, "-->")
}

func TestCallGraphCmd_Focus(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple Go file
	code := `package main
	func main() {
		foo()
	}
	func foo() {
		bar()
	}
	func bar() {}
	`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(code), 0644)
	require.NoError(t, err)

	root := &cobra.Command{Use: "recac"}
	root.AddCommand(callGraphCmd)

	// Reset flags
	callGraphCmd.Flags().Set("dir", tmpDir)
	callGraphCmd.Flags().Set("focus", "foo")

	output, err := executeCommand(root, "callgraph", "--dir", tmpDir, "--focus", "foo")
	assert.NoError(t, err)
	assert.Contains(t, output, "graph LR")

	// Should contain foo and its connections (main -> foo -> bar)
	// Since filter extends 1 level, main calls foo, foo calls bar.
	// So all should be present?
	// main -> foo (main is caller of foo)
	// foo -> bar (bar is callee of foo)
	// So yes.

	// Let's add an unrelated function
	code2 := `package main
	func baz() {}
	`
	err = os.WriteFile(filepath.Join(tmpDir, "other.go"), []byte(code2), 0644)
	require.NoError(t, err)

	// Run again
	output, err = executeCommand(root, "callgraph", "--dir", tmpDir, "--focus", "foo")
	assert.NoError(t, err)

	assert.Contains(t, output, "main.foo")
	assert.NotContains(t, output, "main.baz")
}

func TestGenerateMermaidCallGraph(t *testing.T) {
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"A": {ID: "A", Name: "FuncA"},
			"B": {ID: "B", Name: "FuncB"},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "A", To: "B"},
		},
	}

	output := generateMermaidCallGraph(cg)
	assert.Contains(t, output, "graph LR")
	assert.Contains(t, output, "A[\"A\"]")
	assert.Contains(t, output, "B[\"B\"]")
	assert.Contains(t, output, "A --> B")
}
