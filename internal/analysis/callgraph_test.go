package analysis

import (
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

func TestGenerateCallGraph_Robustness(t *testing.T) {
	// Verify handling of complex receiver types (e.g. multi-parameter generics)
	tmpDir := t.TempDir()

	content := `package pkg

type Multi[K any, V any] struct{}

func (m *Multi[K, V]) Do() {}

func Call() {
    m := &Multi[int, string]{}
    m.Do()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "multi.go"), []byte(content), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Check if node exists and has correct receiver
	// ID should be "pkg.(Multi).Do" (without generic params in name for now, as implemented)
	// If it fails to parse receiver, it might be "pkg.(Unknown).Do"

	found := false
	for id := range cg.Nodes {
		if id == "pkg.(Multi).Do" {
			found = true
			break
		}
	}

	assert.True(t, found, "Should find node pkg.(Multi).Do, checking support for IndexListExpr")
}

func TestGenerateCallGraph_PanicOnNoBody(t *testing.T) {
	tmpDir := t.TempDir()

	// assembly.go contains a forward declaration
	content := `package pkg

func AssemblyFunc()
`
	err := os.WriteFile(filepath.Join(tmpDir, "assembly.go"), []byte(content), 0644)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		_, err := GenerateCallGraph(tmpDir)
		assert.NoError(t, err)
	})
}

func TestGenerateCallGraph_GenericCalls(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package pkg

func GenericFunc[T any]() {}

func Call() {
    GenericFunc[int]()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "generic_call.go"), []byte(content), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Check if edge exists: pkg.Call -> pkg.GenericFunc
	found := false
	for _, edge := range cg.Edges {
		if edge.From == "pkg.Call" && edge.To == "pkg.GenericFunc" {
			found = true
			break
		}
	}

	assert.True(t, found, "Should find edge pkg.Call -> pkg.GenericFunc even with generic instantiation")
}

func TestGenerateCallGraph_Determinism(t *testing.T) {
	tmpDir := t.TempDir()

	// Create multiple files to ensure order matters
	files := map[string]string{
		"a.go": `package pkg; func A() { B() }`,
		"b.go": `package pkg; func B() { C() }`,
		"c.go": `package pkg; func C() {}`,
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Run multiple times
	cg1, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	cg2, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Check Edge Order
	require.Equal(t, len(cg1.Edges), len(cg2.Edges))
	for i := range cg1.Edges {
		assert.Equal(t, cg1.Edges[i], cg2.Edges[i], "Edge at index %d mismatch", i)
	}
}
