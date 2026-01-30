package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGenerateCallGraph_Determinism(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a structure that might be ambiguous
	// pkg/a.go -> package a
	// sub/pkg/a.go -> package a
	// main.go -> imports both (with aliases)
	// imports "example.com/pkg" and "example.com/sub/pkg"

	// Setup files
	files := map[string]string{
		"pkg/a.go":      "package a\nfunc Func() {}",
		"sub/pkg/a.go":  "package a\nfunc Func() {}",
		"nested/b.go":   "package b\nfunc Call() { }",
		"main.go": `package main
import (
	"fmt"
	p1 "example.com/pkg"
	p2 "example.com/sub/pkg"
	"recac/nested"
)
func main() {
	p1.Func()
	p2.Func()
	nested.Call()
	fmt.Println("ok")
}`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Run multiple times
	var firstOutput string
	for i := 0; i < 50; i++ {
		cg, err := GenerateCallGraph(tmpDir)
		require.NoError(t, err)

		// Serialize
		output := serializeGraph(cg)
		if firstOutput == "" {
			firstOutput = output
		} else {
			require.Equal(t, firstOutput, output, "Graph output mismatch at iteration %d", i)
		}
	}
}

func serializeGraph(cg *CallGraph) string {
	// Sort nodes
	var nodes []string
	for _, n := range cg.Nodes {
		nodes = append(nodes, n.ID)
	}
	sort.Strings(nodes)

	// Sort edges
	var edges []string
	for _, e := range cg.Edges {
		edges = append(edges, fmt.Sprintf("%s->%s", e.From, e.To))
	}
	sort.Strings(edges)

	return fmt.Sprintf("Nodes: %v\nEdges: %v", nodes, edges)
}
