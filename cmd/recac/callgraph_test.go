package main

import (
	"bytes"
	"os"
	"path/filepath"
	"recac/internal/analysis"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallGraphCmd_Determinism(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// Create p0 calls p1
	// p0/f.go
	dir0 := filepath.Join(tmpDir, "p0")
	err := os.MkdirAll(dir0, 0755)
	require.NoError(t, err)
	content0 := `package p0
import "recac-test/p1"
func F0() { p1.F1() }`
	err = os.WriteFile(filepath.Join(dir0, "f.go"), []byte(content0), 0644)
	require.NoError(t, err)

	// p1/f.go
	dir1 := filepath.Join(tmpDir, "p1")
	err = os.MkdirAll(dir1, 0755)
	require.NoError(t, err)
	content1 := `package p1
func F1() {}`
	err = os.WriteFile(filepath.Join(dir1, "f.go"), []byte(content1), 0644)
	require.NoError(t, err)

	// Run command
	runCmd := func() string {
		buf := new(bytes.Buffer)

		// Reset global flags
		callGraphDir = tmpDir
		callGraphFocus = ""

		cmd := callGraphCmd
		cmd.SetOut(buf)
		cmd.SetArgs([]string{}) // No args, flags set via variable

		err := runCallGraph(cmd, []string{})
		require.NoError(t, err)
		return buf.String()
	}

	output1 := runCmd()
	output2 := runCmd()

	assert.Equal(t, output1, output2, "Call graph output should be deterministic")
	assert.Contains(t, output1, "graph LR", "Output should be a Mermaid graph")

	// Check for sanitized IDs
	// p0.F0 -> p0_F0 (assuming . replaced by _)
	// p1.F1 -> p1_F1
	assert.Contains(t, output1, "p0_F0", "Should contain sanitized ID for p0.F0")
	assert.Contains(t, output1, "p1_F1", "Should contain sanitized ID for p1.F1")

	// Check edges
	assert.Contains(t, output1, "p0_F0 --> p1_F1", "Should contain edge p0 -> p1")
}

func TestGenerateMermaidCallGraph_Sanitization(t *testing.T) {
	// Manually construct a graph with evil IDs
	cg := &analysis.CallGraph{
		Nodes: map[string]*analysis.CallGraphNode{
			"evil\"id": {ID: "evil\"id", Name: "evil\"name"},
		},
		Edges: []analysis.CallGraphEdge{},
	}

	output := generateMermaidCallGraph(cg)

	// Check label:
	assert.Contains(t, output, "[\"evil'id\"]", "Label should be sanitized")
}
