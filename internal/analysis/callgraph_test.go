package analysis

import (
	"os"
	"path/filepath"
	"sort"
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

func TestResolveExternalCall_Ambiguity(t *testing.T) {
	cg := &CallGraph{
		Nodes: make(map[string]*CallGraphNode),
	}

	// Node 1: pkg/utils.Func
	node1 := &CallGraphNode{
		ID:      "pkg/utils.Func",
		Package: "pkg/utils",
		Name:    "Func",
	}
	cg.Nodes[node1.ID] = node1

	// Node 2: utils.Func (at root)
	node2 := &CallGraphNode{
		ID:      "utils.Func",
		Package: "utils",
		Name:    "Func",
	}
	cg.Nodes[node2.ID] = node2

	// Import path that matches both suffixes
	// Should match pkg/utils because it's longer
	importPath := "github.com/example/pkg/utils"
	funcName := "Func"

	// Prepare nodeIDs
	var nodeIDs []string
	for id := range cg.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	resolvedID := resolveExternalCall(cg, nodeIDs, importPath, funcName)
	assert.Equal(t, "pkg/utils.Func", resolvedID)
}

func TestResolveExternalCall_PartialSuffix(t *testing.T) {
	cg := &CallGraph{
		Nodes: make(map[string]*CallGraphNode),
	}

	// Node: utils.Func
	node := &CallGraphNode{
		ID:      "utils.Func",
		Package: "utils",
		Name:    "Func",
	}
	cg.Nodes[node.ID] = node

	// Import path that ends with "utils" but as part of "autils"
	importPath := "github.com/example/autils"
	funcName := "Func"

	// Prepare nodeIDs
	var nodeIDs []string
	for id := range cg.Nodes {
		nodeIDs = append(nodeIDs, id)
	}
	sort.Strings(nodeIDs)

	resolvedID := resolveExternalCall(cg, nodeIDs, importPath, funcName)
	assert.Equal(t, "", resolvedID, "Should not resolve partial suffix match")
}

func TestGenerateCallGraph_Determinism(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// Create multiple files to trigger map iteration randomness
	files := map[string]string{
		"a.go": "package main\nfunc A() { B() }",
		"b.go": "package main\nfunc B() { C() }",
		"c.go": "package main\nfunc C() { A() }",
		"d.go": "package main\nfunc D() { A() }",
		"e.go": "package main\nfunc E() { B() }",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Run 1
	cg1, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Run 2
	cg2, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Compare Edges (Order matters for slice equality)
	assert.Equal(t, cg1.Edges, cg2.Edges, "Edges should be identical and in the same order")
}

func TestGenerateCallGraph_StressDeterminism(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	files := map[string]string{
		"a.go": "package main\nfunc A() { B() }",
		"b.go": "package main\nfunc B() { C() }",
		"c.go": "package main\nfunc C() { A() }",
	}

	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Run 1 as baseline
	baseline, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Run 50 times
	for i := 0; i < 50; i++ {
		cg, err := GenerateCallGraph(tmpDir)
		require.NoError(t, err)
		assert.Equal(t, baseline.Edges, cg.Edges, "Edges mismatch at iteration %d", i)
	}
}
