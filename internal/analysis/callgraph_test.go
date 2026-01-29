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

func TestGenerateCallGraph_Determinism(t *testing.T) {
	// Create a complex enough structure to potentially trigger map iteration randomness
	tmpDir := t.TempDir()

	for i := 0; i < 10; i++ {
		content := fmt.Sprintf("package p%d\nfunc F%d() {}", i, i)
		dir := filepath.Join(tmpDir, fmt.Sprintf("p%d", i))
		os.MkdirAll(dir, 0755)
		os.WriteFile(filepath.Join(dir, "f.go"), []byte(content), 0644)
	}

	// First run
	cg1, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Second run
	cg2, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Check edges order
	require.Equal(t, len(cg1.Edges), len(cg2.Edges))

	// If edges are empty this test is trivial, but here we only defined functions, no calls.
	// We need calls to test edge ordering.

	// Let's add calls.
	// p0 calls p1, p1 calls p2 ...
	for i := 0; i < 9; i++ {
		content := fmt.Sprintf(`package p%d
import "recac-test/p%d"
func F%d() { p%d.F%d() }`, i, i+1, i, i+1, i+1)
		dir := filepath.Join(tmpDir, fmt.Sprintf("p%d", i))
		os.WriteFile(filepath.Join(dir, "f.go"), []byte(content), 0644)
	}
	// Last one
	os.WriteFile(filepath.Join(tmpDir, "p9", "f.go"), []byte("package p9\nfunc F9() {}"), 0644)

	cg3, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)
	cg4, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Serialize edges to compare order
	edges3 := fmt.Sprintf("%v", cg3.Edges)
	edges4 := fmt.Sprintf("%v", cg4.Edges)

	assert.Equal(t, edges3, edges4, "Call graph edges should be deterministic")
}

func TestGenerateCallGraph_ParentDir(t *testing.T) {
	tmpDir := t.TempDir()
	// structure:
	// tmpDir/
	//   subdir/
	//     main.go

	subdir := filepath.Join(tmpDir, "subdir")
	os.Mkdir(subdir, 0755)

	err := os.WriteFile(filepath.Join(subdir, "main.go"), []byte("package main\nfunc Main() {}"), 0644)
	require.NoError(t, err)

	// Run from tmpDir, pointing to "subdir/.."
	// which resolves to tmpDir.

	cwd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(cwd)
	os.Chdir(subdir)

	// Now we are in subdir. We want to analyze "..".
	cg, err := GenerateCallGraph("..")
	require.NoError(t, err)

	// Should find Main
	found := false
	for _, node := range cg.Nodes {
		if node.Name == "Main" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find function in parent directory when analyzing '..'")
}

func TestGenerateCallGraph_GenericCalls(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main
func G[T any]() {}
func Main() {
	G[int]()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Check for edge Main -> G
	found := false
	for _, edge := range cg.Edges {
		if edge.From == "main.Main" && edge.To == "main.G" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should find edge to generic function call")
}

func TestResolveExternalCall_Ambiguity(t *testing.T) {
	tmpDir := t.TempDir()

	// Package 1: "a/b" (suffix "a/b")
	// Function Target()
	dir3 := filepath.Join(tmpDir, "a", "b")
	os.MkdirAll(dir3, 0755)
	os.WriteFile(filepath.Join(dir3, "f.go"), []byte("package b\nfunc Target() {}"), 0644)

	// Package 2: "c/a/b" (suffix "c/a/b" AND "a/b")
	// Function Target()
	dir4 := filepath.Join(tmpDir, "c", "a", "b")
	os.MkdirAll(dir4, 0755)
	os.WriteFile(filepath.Join(dir4, "f.go"), []byte("package b\nfunc Target() {}"), 0644)

	// Main calls "b.Target()" with import "x/c/a/b"
	// Import path "x/c/a/b" ends with "a/b" (Package 1)
	// Import path "x/c/a/b" ends with "c/a/b" (Package 2)
	// So both are candidates.

	dirMain := filepath.Join(tmpDir, "main")
	os.MkdirAll(dirMain, 0755)
	content := `package main
import (
	b "x/c/a/b"
)
func Main() {
	b.Target()
}
`
	os.WriteFile(filepath.Join(dirMain, "main.go"), []byte(content), 0644)

	// We expect determinism.
	// Run multiple times.

	var firstTarget string

	for i := 0; i < 20; i++ {
		cg, err := GenerateCallGraph(tmpDir)
		require.NoError(t, err)

		// Find edge from Main
		var target string
		for _, edge := range cg.Edges {
			if edge.From == "main.Main" {
				target = edge.To
				break
			}
		}

		// We expect the LONGEST suffix match to be chosen.
		// Import "x/c/a/b" ends with "c/a/b" (package b in dir4).
		// ID for dir4 is "c/a/b.Target" (relative to tmpDir).
		expectedTarget := filepath.ToSlash(filepath.Join("c", "a", "b")) + ".Target"

		if firstTarget == "" {
			firstTarget = target
		} else {
			assert.Equal(t, firstTarget, target, "Found different target for ambiguous call")
		}

		assert.Equal(t, expectedTarget, target, "Should resolve to longest matching suffix")
	}
}
