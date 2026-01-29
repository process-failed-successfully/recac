package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/analysis"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCallGraph(t *testing.T) {
	// Setup Temp Dir
	tempDir := t.TempDir()
	code := `package main

func main() {
	foo()
}

func foo() {
	bar()
}

func bar() {}
`
	err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(code), 0644)
	require.NoError(t, err)

	// Set flags
	cmd := callGraphCmd
	cmd.Flags().Set("dir", tempDir)
	cmd.Flags().Set("focus", "")

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Run
	err = runCallGraph(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "graph LR")
	assert.Contains(t, output, "main_main --> main_foo")
	assert.Contains(t, output, "main_foo --> main_bar")
}

func TestRunCallGraph_Focus(t *testing.T) {
	// Setup Temp Dir
	tempDir := t.TempDir()
	code := `package main

func main() {
	foo()
}

func foo() {
	bar()
}

func bar() {}
`
	err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(code), 0644)
	require.NoError(t, err)

	// Set flags
	cmd := callGraphCmd
	cmd.Flags().Set("dir", tempDir)
	cmd.Flags().Set("focus", "foo")

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Run
	err = runCallGraph(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "main_main --> main_foo")
	assert.Contains(t, output, "main_foo --> main_bar")
	// Should NOT contain random stuff if we had more, but here it's small.
}

func TestFilterGraph(t *testing.T) {
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"a": {ID: "a", Name: "Alpha"},
			"b": {ID: "b", Name: "Beta"},
			"c": {ID: "c", Name: "Charlie"}, // Disconnected
			"d": {ID: "d", Name: "Delta"},
		},
		Edges: []analysis.CallGraphEdge{
			{From: "a", To: "b"},
			{From: "b", To: "d"},
		},
	}

	// Filter "Alpha"
	filtered := filterGraph(cg, "Alpha")
	require.NotNil(t, filtered)

	// Should include Alpha (match), Beta (callee of A)
	assert.Contains(t, filtered.Nodes, "a")
	assert.Contains(t, filtered.Nodes, "b")
	assert.NotContains(t, filtered.Nodes, "c")
	assert.NotContains(t, filtered.Nodes, "d") // Beta calls Delta, but we only expand 1 level?
	// Logic in filterGraph:
	// 1. Find relevant nodes (Alpha -> a)
	// 2. Expand to 1 level (callers and callees).
	// Edge a->b involves a. So b is added.
	// Edge b->d involves b. b is NOT in relevantNodes initially, only in expandedNodes?
	// Wait, let's check code:
	// "for _, edge := range cg.Edges { if relevantNodes[edge.From] || relevantNodes[edge.To] ..."
	// relevantNodes contains only "a".
	// Edge a->b: From is "a", so yes. Adds b.
	// Edge b->d: From is "b", To is "d". Neither is in relevantNodes ("a"). So NO.

	assert.Len(t, filtered.Edges, 1)
	assert.Equal(t, "a", filtered.Edges[0].From)
	assert.Equal(t, "b", filtered.Edges[0].To)
}
