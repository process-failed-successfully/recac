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

func TestGenerateCallGraph_Generics(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main

type MyGeneric[T any] struct{}

func (g *MyGeneric[T]) DoOne() {}

type MyMultiGeneric[K, V any] struct{}

func (g *MyMultiGeneric[K, V]) DoTwo() {}

func main() {
	g1 := &MyGeneric[int]{}
	g1.DoOne()

	g2 := &MyMultiGeneric[int, string]{}
	g2.DoTwo()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	nodeIDs := make(map[string]bool)
	for id := range cg.Nodes {
		nodeIDs[id] = true
	}

	// Check if methods are correctly identified with receiver type name
	// MyGeneric[T] -> MyGeneric (via IndexExpr)
	// MyMultiGeneric[K, V] -> MyMultiGeneric (via IndexListExpr)

	assert.Contains(t, nodeIDs, "main.(MyGeneric).DoOne")
	assert.Contains(t, nodeIDs, "main.(MyMultiGeneric).DoTwo")
}

func TestGenerateCallGraph_PanicOnNoBody(t *testing.T) {
	tmpDir := t.TempDir()

	// Function without body (forward declaration)
	content := `package main
func ForwardDecl()
func main() {
	ForwardDecl()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)
	require.NotNil(t, cg)

	// Should contain the node
	assert.Contains(t, cg.Nodes, "main.ForwardDecl")
}

func TestGenerateCallGraph_GenericFuncCall(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main

func GenericFunc[T any]() {}

func main() {
	GenericFunc[int]()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Check edge: main -> GenericFunc
	found := false
	for _, edge := range cg.Edges {
		if edge.From == "main.main" && edge.To == "main.GenericFunc" {
			found = true
			break
		}
	}
	assert.True(t, found, "Missing edge to generic function instantiation")
}

func TestGenerateCallGraph_Determinism(t *testing.T) {
	// Create a complex setup to ensure non-trivial traversal
	tmpDir := t.TempDir()

	// File 1: A.go
	err := os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte(`package main
func A() { B() }
`), 0644)
	require.NoError(t, err)

	// File 2: B.go
	err = os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte(`package main
func B() { C() }
`), 0644)
	require.NoError(t, err)

	// File 3: C.go
	err = os.WriteFile(filepath.Join(tmpDir, "c.go"), []byte(`package main
func C() { A() }
`), 0644)
	require.NoError(t, err)

	// Run 1
	cg1, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Run 2
	cg2, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Verify Edges match exactly in order
	require.Equal(t, len(cg1.Edges), len(cg2.Edges), "Edge count mismatch")
	for i := range cg1.Edges {
		assert.Equal(t, cg1.Edges[i], cg2.Edges[i], "Edge mismatch at index %d", i)
	}
}

func TestResolveExternalCall_Ambiguity(t *testing.T) {
	// Setup temporary directory
	tmpDir := t.TempDir()

	// 1. Create utils/helper.go -> utils.Helper
	// ID: utils.Helper
	utilsDir := filepath.Join(tmpDir, "utils")
	err := os.MkdirAll(utilsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(utilsDir, "helper.go"), []byte(`package utils
func Helper() {}
`), 0644)
	require.NoError(t, err)

	// 2. Create zutils/helper.go -> zutils.Helper
	// ID: zutils.Helper
	// zutils > utils, so utils comes first in sorted list.
	zutilsDir := filepath.Join(tmpDir, "zutils")
	err = os.MkdirAll(zutilsDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(zutilsDir, "helper.go"), []byte(`package zutils
func Helper() {}
`), 0644)
	require.NoError(t, err)

	// 3. Create main.go importing "example.com/project/zutils"
	// We want to ensure it resolves to zutils.Helper, NOT utils.Helper
	// utils is a suffix of zutils.
	// utils comes before zutils in sort.
	mainContent := `package main

import (
	"example.com/project/zutils"
)

func main() {
	zutils.Helper()
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Verify Edge
	// main.main -> zutils.Helper
	// NOT utils.Helper

	foundCorrect := false
	foundIncorrect := false

	// IDs are "relPath/Package.Func"
	// utils -> "utils.Helper"
	// zutils -> "zutils.Helper"

	for _, edge := range cg.Edges {
		if edge.From == "main.main" {
			if edge.To == "zutils.Helper" {
				foundCorrect = true
			}
			if edge.To == "utils.Helper" {
				foundIncorrect = true
			}
		}
	}

	assert.True(t, foundCorrect, "Should resolve to zutils.Helper")
	assert.False(t, foundIncorrect, "Should NOT resolve to utils.Helper (partial suffix match)")
}
