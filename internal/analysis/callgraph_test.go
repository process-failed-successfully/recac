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

func TestGenerateCallGraph_StrictDeterminism(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// 1. Create main.go
	mainContent := `package main

import "fmt"

func main() {
	fmt.Println("Hello")
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// 2. Create ignored directories
	// vendor/
	err = os.MkdirAll(filepath.Join(tmpDir, "vendor"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "vendor", "ignored.go"), []byte("package ignored\nfunc Ignored() {}"), 0644)
	require.NoError(t, err)

	// testdata/
	err = os.MkdirAll(filepath.Join(tmpDir, "testdata"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "testdata", "data.go"), []byte("package data\nfunc Data() {}"), 0644)
	require.NoError(t, err)

	// Run multiple times and compare results
	var firstRun *CallGraph

	for i := 0; i < 5; i++ {
		cg, err := GenerateCallGraph(tmpDir)
		require.NoError(t, err)
		require.NotNil(t, cg)

		// Verify ignored files are not in nodes
		for id := range cg.Nodes {
			assert.NotContains(t, id, "ignored", "Should ignore vendor directory")
			assert.NotContains(t, id, "data", "Should ignore testdata directory")
		}

		if firstRun == nil {
			firstRun = cg
		} else {
			// Compare edges strictly
			require.Equal(t, len(firstRun.Edges), len(cg.Edges), "Edge count mismatch")
			for j, edge := range firstRun.Edges {
				require.Equal(t, edge.From, cg.Edges[j].From, "Edge From mismatch at index %d", j)
				require.Equal(t, edge.To, cg.Edges[j].To, "Edge To mismatch at index %d", j)
			}
		}
	}
}
