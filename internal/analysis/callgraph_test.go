package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateCallGraph(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// 1. Create main.go
	// It calls pkg.Helper() and fmt.Println()
	mainContent := `package main

import (
	"fmt"
	"recac-test/pkg"
)

func main() {
	pkg.Helper()
	fmt.Println("Done")
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// 2. Create pkg/helper.go
	pkgDir := filepath.Join(tmpDir, "pkg")
	err = os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)

	// Helper calls s.DoWork(), s is Service.
	// Service.DoWork calls internalFunc.
	pkgContent := `package pkg

type Service struct{}

func (s *Service) DoWork() {
	internalFunc()
}

func Helper() {
	s := &Service{}
	s.DoWork()
}

func internalFunc() {}
`
	err = os.WriteFile(filepath.Join(pkgDir, "helper.go"), []byte(pkgContent), 0644)
	require.NoError(t, err)

	// Run Analysis
	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, cg)

	// --- Verify Nodes ---
	// Expected Nodes:
	// main.main
	// pkg.Helper
	// pkg.(Service).DoWork
	// pkg.internalFunc

	// Note: Our IDs are "relPath/Package.Func".
	// For main.go: ".main.main" (if in root)
	// For pkg/helper.go: "pkg.Helper"

	nodeIDs := make(map[string]bool)
	for id := range cg.Nodes {
		nodeIDs[id] = true
	}

	assert.Contains(t, nodeIDs, "main.main", "Should contain main function")
	assert.Contains(t, nodeIDs, "pkg.Helper", "Should contain Helper function")
	assert.Contains(t, nodeIDs, "pkg.(Service).DoWork", "Should contain Service method")
	assert.Contains(t, nodeIDs, "pkg.internalFunc", "Should contain internal function")

	// --- Verify Edges ---

	// Edge 1: main.main -> pkg.Helper
	foundMainToHelper := false
	// Edge 2: pkg.Helper -> pkg.(Service).DoWork
	foundHelperToDoWork := false
	// Edge 3: pkg.(Service).DoWork -> pkg.internalFunc
	foundDoWorkToInternal := false

	for _, edge := range cg.Edges {
		if edge.From == "main.main" && edge.To == "pkg.Helper" {
			foundMainToHelper = true
		}
		// Note: The heuristic might resolve s.DoWork() to pkg.(Service).DoWork if it's unique
		if edge.From == "pkg.Helper" && edge.To == "pkg.(Service).DoWork" {
			foundHelperToDoWork = true
		}
		if edge.From == "pkg.(Service).DoWork" && edge.To == "pkg.internalFunc" {
			foundDoWorkToInternal = true
		}
	}

	assert.True(t, foundMainToHelper, "Missing edge: main -> Helper")
	assert.True(t, foundHelperToDoWork, "Missing edge: Helper -> DoWork")
	assert.True(t, foundDoWorkToInternal, "Missing edge: DoWork -> internalFunc")
}

func TestGenerateCallGraph_WithForwardDecl(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// Create a file with a function forward declaration (no body)
	content := `package pkg

func ExternalFunc()
`
	err := os.WriteFile(filepath.Join(tmpDir, "external.go"), []byte(content), 0644)
	require.NoError(t, err)

	// Run Analysis
	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, cg)

	// Should contain the node, but no panic
	nodeIDs := make(map[string]bool)
	for id := range cg.Nodes {
		nodeIDs[id] = true
	}
	assert.Contains(t, nodeIDs, "pkg.ExternalFunc")
}

func TestGenerateCallGraph_Stress(t *testing.T) {
	tmpDir := t.TempDir()

	files := map[string]string{
		"1.go": "package p; func F()",
		"2.go": "package p; type L[T any] []T; func (l L[T]) M() {}",
		"3.go": "package p; type L[T any] []T; func (l *L[T]) M() {}",
		"5.go": "package p",
		"6.go": "// Just comment\npackage p",
		"7.go": "package p; func {", // Syntax error
		"8.go": `package p
		func Outer() {
			func() {
				Inside()
			}()
		}
		func Inside() {}`,
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, cg)
}

func TestGenerateCallGraph_StrictDeterminism(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple files to trigger potential map iteration randomness
	for i := 0; i < 20; i++ {
		content := fmt.Sprintf(`package p%d
		import "fmt"
		func Func%d() {
			fmt.Println("Hi")
			Func%d()
		}
		`, i, i, (i+1)%20)
		err := os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("%d.go", i)), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Run multiple times and compare results
	var firstOutput string
	for i := 0; i < 5; i++ {
		cg, err := GenerateCallGraph(tmpDir)
		require.NoError(t, err)

		// Serialize Edges to string for comparison
		output := ""
		for _, edge := range cg.Edges {
			output += fmt.Sprintf("%s->%s\n", edge.From, edge.To)
		}

		if i == 0 {
			firstOutput = output
			// Check that we actually generated something to make the test meaningful
			if output == "" {
				t.Error("Warning: No edges generated in StrictDeterminism test")
			}
		} else {
			assert.Equal(t, firstOutput, output, "Output should be deterministic across runs")
		}
	}
}
