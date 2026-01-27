package analysis

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCallGraph_NoBodyPanic(t *testing.T) {
	// Regression test for issue where functions without body (e.g. forward declarations) caused a panic.

	tmpDir := t.TempDir()

	// Create a file with a forward declaration (no body)
	content := `package pkg

func DeclaredOnly()
`
	err := os.WriteFile(filepath.Join(tmpDir, "decl.go"), []byte(content), 0644)
	require.NoError(t, err)

	// Run Analysis
	// It should not panic and return no error (assuming file is parsed correctly or ignored)
	cg, err := GenerateCallGraph(tmpDir)

	// We expect no error because parser.ParseFile handles this valid Go syntax.
	assert.NoError(t, err)
	assert.NotNil(t, cg)

	// Verify that the node is present in the graph (even if it has no calls)
	// ID should be "pkg.DeclaredOnly" (assuming file is in tmpDir/decl.go, so package is "pkg", path is ".")
	// Since GenerateCallGraph logic for fullPkg:
	// relDir = "." -> fullPkg = pkgName = "pkg"
	// So ID = "pkg.DeclaredOnly"

	_, exists := cg.Nodes["pkg.DeclaredOnly"]
	assert.True(t, exists, "Function without body should still be indexed as a node")
}
