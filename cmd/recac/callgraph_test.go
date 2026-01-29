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

	// Use factory to get a fresh command
	cmd := NewCallGraphCmd()
	cmd.Flags().Set("dir", tempDir)
	cmd.Flags().Set("focus", "")

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Run
	// Note: We shouldn't invoke runCallGraph directly if we want to test flag parsing,
	// but here we are setting flags and running the RunE.
	// RunE expects *cobra.Command and []string.
	err = cmd.RunE(cmd, []string{})
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

	// Use factory
	cmd := NewCallGraphCmd()
	cmd.Flags().Set("dir", tempDir)
	cmd.Flags().Set("focus", "foo")

	// Capture output
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Run
	err = cmd.RunE(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()
	assert.Contains(t, output, "main_main --> main_foo")
	assert.Contains(t, output, "main_foo --> main_bar")
}

func TestRunCallGraph_Error(t *testing.T) {
	// Use factory
	cmd := NewCallGraphCmd()
	// Non-existent directory
	cmd.Flags().Set("dir", "/non/existent/path/definitely")

	// Capture output (though we expect error)
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)

	// Run
	err := cmd.RunE(cmd, []string{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to generate call graph")
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
	assert.NotContains(t, filtered.Nodes, "d") // Beta calls Delta, but we only expand 1 level

	assert.Len(t, filtered.Edges, 1)
	assert.Equal(t, "a", filtered.Edges[0].From)
	assert.Equal(t, "b", filtered.Edges[0].To)
}
