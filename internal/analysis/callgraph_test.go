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

func TestGenerateCallGraph_StrictImportMatching(t *testing.T) {
	// Reproduction for non-deterministic import matching
	tmpDir := t.TempDir()

	// Create "pkg" and "apkg"
	// "apkg" has suffix "pkg".
	// import "example.com/apkg" should match "apkg", NOT "pkg".

	// 1. Create pkg/lib.go
	pkgDir := filepath.Join(tmpDir, "pkg")
	err := os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)
	os.WriteFile(filepath.Join(pkgDir, "lib.go"), []byte("package pkg\nfunc Do() {}"), 0644)

	// 2. Create apkg/lib.go
	apkgDir := filepath.Join(tmpDir, "apkg")
	err = os.MkdirAll(apkgDir, 0755)
	require.NoError(t, err)
	os.WriteFile(filepath.Join(apkgDir, "lib.go"), []byte("package apkg\nfunc Do() {}"), 0644)

	// 3. Create main.go importing apkg
	mainContent := `package main
import "example.com/apkg"
func main() {
	apkg.Do()
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Run Analysis
	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Verify edge
	// Should be main.main -> apkg.Do
	// NOT main.main -> pkg.Do

	foundCorrect := false
	foundWrong := false

	for _, edge := range cg.Edges {
		if edge.From == "main.main" {
			if edge.To == "apkg.Do" {
				foundCorrect = true
			}
			if edge.To == "pkg.Do" {
				foundWrong = true
			}
		}
	}

	assert.True(t, foundCorrect, "Should find edge to apkg.Do")
	assert.False(t, foundWrong, "Should NOT find edge to pkg.Do")
}
