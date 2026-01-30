package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCallGraph_Determinism(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// Create multiple files to ensure iteration order matters
	files := map[string]string{
		"main.go": `package main
import "recac-test/pkg"
func main() { pkg.A(); pkg.B() }`,
		"pkg/a.go": `package pkg
func A() { B() }`,
		"pkg/b.go": `package pkg
func B() { A() }`,
		"pkg/c.go": `package pkg
func C() {}`,
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	// Run analysis multiple times
	var firstCG *CallGraph
	for i := 0; i < 10; i++ {
		cg, err := GenerateCallGraph(tmpDir)
		require.NoError(t, err)

		if i == 0 {
			firstCG = cg
		} else {
			// Compare Nodes
			assert.Equal(t, len(firstCG.Nodes), len(cg.Nodes), "Run %d: Node count mismatch", i)
			for id, node := range firstCG.Nodes {
				otherNode, exists := cg.Nodes[id]
				assert.True(t, exists, "Run %d: Node %s missing", i, id)
				assert.Equal(t, node, otherNode, "Run %d: Node %s mismatch", i, id)
			}

			// Compare Edges
			// Edges is a slice, so order matters for DeepEqual?
			// Wait, the order SHOULD be deterministic if our code is correct.
			// However, `cg.Edges` is appended to in `resolveCalls`.
			// If `resolveCalls` is deterministic, `cg.Edges` order is deterministic.
			assert.Equal(t, firstCG.Edges, cg.Edges, "Run %d: Edges mismatch", i)
		}
	}
}
