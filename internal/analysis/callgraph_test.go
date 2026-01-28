package analysis

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	nodeIDs := make(map[string]bool)
	for id := range cg.Nodes {
		nodeIDs[id] = true
		// Verify normalization
		assert.False(t, strings.Contains(id, "\\"), "Node ID should not contain backslashes: %s", id)
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

func TestGenerateCallGraph_StrictDeterminism(t *testing.T) {
	// Create a complex structure with many files to increase chance of map iteration randomness affecting order
	tmpDir := t.TempDir()

	for i := 0; i < 20; i++ {
		content := fmt.Sprintf(`package p%d
		func F%d() {
			F%d()
		}`, i, i, i+1)
		err := os.WriteFile(filepath.Join(tmpDir, fmt.Sprintf("f%d.go", i)), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Run twice
	cg1, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	cg2, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Compare Edges
	require.Equal(t, len(cg1.Edges), len(cg2.Edges))

	for i := range cg1.Edges {
		assert.Equal(t, cg1.Edges[i], cg2.Edges[i], "Edges at index %d mismatch", i)
	}
}

func TestGenerateCallGraph_SkipsIgnoredDirs(t *testing.T) {
	tmpDir := t.TempDir()

	// Valid code in root
	err := os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte("package main\nfunc Root(){}"), 0644)
	require.NoError(t, err)

	// Vendor directory
	vendorDir := filepath.Join(tmpDir, "vendor")
	err = os.Mkdir(vendorDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(vendorDir, "dep.go"), []byte("package dep\nfunc Dep(){}"), 0644)
	require.NoError(t, err)

	// Testdata directory
	testdataDir := filepath.Join(tmpDir, "testdata")
	err = os.Mkdir(testdataDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(testdataDir, "bad.go"), []byte("package bad\nfunc Bad(){}"), 0644)
	require.NoError(t, err)

	// node_modules directory
	nodeModulesDir := filepath.Join(tmpDir, "node_modules")
	err = os.Mkdir(nodeModulesDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(nodeModulesDir, "js.go"), []byte("package js\nfunc JS(){}"), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Check that ignored files are NOT in the graph
	for id := range cg.Nodes {
		assert.NotContains(t, id, "vendor", "Should not contain vendor nodes")
		assert.NotContains(t, id, "testdata", "Should not contain testdata nodes")
		assert.NotContains(t, id, "node_modules", "Should not contain node_modules nodes")
		assert.NotContains(t, id, "Dep")
		assert.NotContains(t, id, "Bad")
		assert.NotContains(t, id, "JS")
	}

	assert.Contains(t, cg.Nodes, "main.Root", "Should contain root node")
}
