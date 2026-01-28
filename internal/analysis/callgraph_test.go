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

func TestGenerateCallGraph_PanicOnNilBody(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a Go file with a function declaration but no body (like in assembly)
	content := `package pkg
func AssemblyFunc()
`
	pkgDir := filepath.Join(tmpDir, "pkg")
	err := os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(pkgDir, "assembly.go"), []byte(content), 0644)
	require.NoError(t, err)

	_, err = GenerateCallGraph(tmpDir)
	require.NoError(t, err)
}

func TestGenerateCallGraph_IgnoreDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// vendor/pkg/ignored.go
	vendorDir := filepath.Join(tmpDir, "vendor", "pkg")
	err := os.MkdirAll(vendorDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(vendorDir, "ignored.go"), []byte("package pkg\nfunc Ignored() {}"), 0644)
	require.NoError(t, err)

	// testdata/pkg/ignored.go
	testdataDir := filepath.Join(tmpDir, "testdata", "pkg")
	err = os.MkdirAll(testdataDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(testdataDir, "ignored.go"), []byte("package pkg\nfunc Ignored() {}"), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	for id := range cg.Nodes {
		assert.NotContains(t, id, "vendor", "Should not contain vendor nodes")
		assert.NotContains(t, id, "testdata", "Should not contain testdata nodes")
	}
}

func TestResolveExternalCall_Determinism(t *testing.T) {
	// Create a graph with ambiguous packages
	cg := &CallGraph{
		Nodes: map[string]*CallGraphNode{
			"pkg.Func": {
				ID: "pkg.Func", Package: "pkg", Name: "Func",
			},
			"sub/pkg.Func": {
				ID: "sub/pkg.Func", Package: "sub/pkg", Name: "Func",
			},
			"other/pkg.Func": {
				ID: "other/pkg.Func", Package: "other/pkg", Name: "Func",
			},
			// Test tie-breaking by ID
			"b/pkg.Func": {
				ID: "b/pkg.Func", Package: "b/pkg", Name: "Func",
			},
			"a/pkg.Func": {
				ID: "a/pkg.Func", Package: "b/pkg", Name: "Func", // Same package "b/pkg"
			},
		},
	}

	// 1. Longest match wins
	// "sub/pkg.Func" (len 7) vs "pkg.Func" (len 3)
	// Import: "example.com/sub/pkg"
	id := resolveExternalCall(cg, "example.com/sub/pkg", "Func")
	assert.Equal(t, "sub/pkg.Func", id, "Should select longest suffix match")

	// 2. ID Tie-breaking
	// Import: "example.com/b/pkg"
	// Matches "b/pkg" (used by "b/pkg.Func" and "a/pkg.Func")
	// "a/pkg.Func" < "b/pkg.Func"
	id = resolveExternalCall(cg, "example.com/b/pkg", "Func")
	assert.Equal(t, "a/pkg.Func", id, "Should break ties using ID (lexicographically)")
}
