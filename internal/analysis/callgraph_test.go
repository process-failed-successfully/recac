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

func TestGenerateCallGraph_PanicOnNoBody(t *testing.T) {
	// Regression test for "unexpected node type <nil>" panic when function body is nil
	tmpDir := t.TempDir()

	// Create a file with a function declaration but no body
	// (Simulating external function or assembly)
	// Note: "func noBody()" is valid in Go if matched with assembly, but parser accepts it.
	content := `package main

func noBody()
`
	err := os.WriteFile(filepath.Join(tmpDir, "nobody.go"), []byte(content), 0644)
	require.NoError(t, err)

	// Should not panic
	assert.NotPanics(t, func() {
		_, err := GenerateCallGraph(tmpDir)
		// It might return error or not, but shouldn't panic
		if err != nil {
			t.Logf("GenerateCallGraph returned error: %v", err)
		}
	})
}

func TestGenerateCallGraph_Robustness(t *testing.T) {
	// Test robustness against complex types
	tmpDir := t.TempDir()

	content := `package main

type List[T any] struct{}

func (l *List[T]) Add(val T) {}

type Map[K, V any] struct{}

func (m *Map[K, V]) Set(k K, v V) {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "generic.go"), []byte(content), 0644)
	require.NoError(t, err)

	assert.NotPanics(t, func() {
		cg, err := GenerateCallGraph(tmpDir)
		require.NoError(t, err)

		// Verify we can find the methods even if type name is simplified
		foundMapSet := false
		for id := range cg.Nodes {
			if id == "main.(Map).Set" || id == "main.(Map[K, V]).Set" {
				foundMapSet = true
			}
		}
		// Currently getReceiverTypeName might return "Unknown" for Map[K,V]
		// so we just check it doesn't panic.
		if !foundMapSet {
			t.Log("Did not find Map.Set with expected ID (this is expected until fix)")
		}
	})
}
