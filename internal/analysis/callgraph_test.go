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

func TestResolveExternalCall_Ambiguous(t *testing.T) {
	// Verifies determinism when packages overlap in suffix
	tmpDir := t.TempDir()

	// 1. Create pkg/func.go (path: pkg)
	pkgDir := filepath.Join(tmpDir, "pkg")
	err := os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(pkgDir, "func.go"), []byte("package pkg\nfunc Func() {}"), 0644)
	require.NoError(t, err)

	// 2. Create sub/pkg/func.go (path: sub/pkg)
	subPkgDir := filepath.Join(tmpDir, "sub", "pkg")
	err = os.MkdirAll(subPkgDir, 0755)
	require.NoError(t, err)

	err = os.WriteFile(filepath.Join(subPkgDir, "func.go"), []byte("package pkg\nfunc Func() {}"), 0644)
	require.NoError(t, err)

	// 3. Create main.go calling sub/pkg
	// We want to verify it resolves to sub/pkg.Func, not pkg.Func
	mainContent := `package main
import (
	"fmt"
	"example.com/sub/pkg"
)
func main() {
	pkg.Func()
	fmt.Println("ok")
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Run Analysis
	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Check edge from main.main -> sub/pkg.Func
	found := false
	for _, edge := range cg.Edges {
		if edge.From == "main.main" && edge.To == "sub/pkg.Func" {
			found = true
			break
		}
	}
	assert.True(t, found, "Should resolve to sub/pkg.Func (longer match) instead of pkg.Func")
}

func TestGenerateCallGraph_ExternalCallAmbiguity(t *testing.T) {
	tmpDir := t.TempDir()

	// 1. Create short/match.go -> produces package path "short/match"
	path1 := filepath.Join(tmpDir, "short/match.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(path1), 0755))
	require.NoError(t, os.WriteFile(path1, []byte("package match\nfunc Do() {}"), 0644))

	// 2. Create long/short/match.go -> produces package path "long/short/match"
	path2 := filepath.Join(tmpDir, "long/short/match.go")
	require.NoError(t, os.MkdirAll(filepath.Dir(path2), 0755))
	require.NoError(t, os.WriteFile(path2, []byte("package match\nfunc Do() {}"), 0644))

	// 3. Create main.go importing "example.com/long/short/match"
	// This import path ends with BOTH "short/match" and "long/short/match"
	mainContent := `package main
import (
	"fmt"
	m "example.com/long/short/match"
)
func main() {
	m.Do()
	fmt.Println("Done")
}
`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644))

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Verify that the call is resolved to "long/short/match.Do" (the more specific match)
	foundSpecific := false
	foundAmbiguous := false

	for _, edge := range cg.Edges {
		if edge.From == "main.main" {
			if edge.To == "long/short/match.Do" {
				foundSpecific = true
			} else if edge.To == "short/match.Do" {
				foundAmbiguous = true
			}
		}
	}

	assert.True(t, foundSpecific, "Should resolve to long/short/match.Do")
	assert.False(t, foundAmbiguous, "Should NOT resolve to short/match.Do")
}
