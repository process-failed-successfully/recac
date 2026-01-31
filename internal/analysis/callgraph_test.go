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

func TestResolveExternalCall_StrictSuffix(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// 1. Create main.go which imports "example.com/foopkg"
	// But we have a local package "pkg"
	// We want to make sure calling foopkg.Func() does NOT resolve to pkg.Func()

	mainContent := `package main

import (
	"fmt"
	"example.com/foopkg"
)

func main() {
	foopkg.DoSomething()
	fmt.Println("Done")
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// 2. Create local package "pkg"
	pkgDir := filepath.Join(tmpDir, "pkg")
	err = os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)

	pkgContent := `package pkg

func DoSomething() {}
`
	err = os.WriteFile(filepath.Join(pkgDir, "lib.go"), []byte(pkgContent), 0644)
	require.NoError(t, err)

	// Run Analysis
	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Verify that main.main calls "example.com/foopkg.DoSomething" (external)
	// AND NOT "pkg.DoSomething" (local)

	foundIncorrectLink := false
	for _, edge := range cg.Edges {
		if edge.From == "main.main" && edge.To == "pkg.DoSomething" {
			foundIncorrectLink = true
		}
	}

	assert.False(t, foundIncorrectLink, "Should not resolve foopkg.DoSomething to pkg.DoSomething")
}

func TestResolveExternalCall_AmbiguousSuffix(t *testing.T) {
	// Scenario:
	// main.go imports "example.com/project/long/path/utils"
	// We have local packages:
	// 1. "utils" (at root/utils)
	// 2. "path/utils" (at root/path/utils)
	// Both have "Helper()" function.
	// The import should match "path/utils" better than "utils"?
	// Or actually, wait.
	// If import is "example.com/project/long/path/utils".
	// Local package 1: "utils". pathLen=len("...utils"), pkgLen=5. Suffix matches. Slash check? "...utils"[len-5-1] is '/'. Match!
	// Local package 2: "path/utils". Matches too.
	// We should pick the Longest Match (most specific).

	tmpDir := t.TempDir()

	// main.go
	mainContent := `package main

import (
	"fmt"
	"example.com/project/long/path/utils"
)

func main() {
	utils.Helper()
	fmt.Println("Done")
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// 1. Create utils (shallow)
	utilsDir := filepath.Join(tmpDir, "utils")
	err = os.MkdirAll(utilsDir, 0755)
	require.NoError(t, err)
	os.WriteFile(filepath.Join(utilsDir, "utils.go"), []byte("package utils\nfunc Helper() {}"), 0644)

	// 2. Create path/utils (deeper)
	deepUtilsDir := filepath.Join(tmpDir, "path", "utils")
	err = os.MkdirAll(deepUtilsDir, 0755)
	require.NoError(t, err)
	os.WriteFile(filepath.Join(deepUtilsDir, "utils.go"), []byte("package utils\nfunc Helper() {}"), 0644)

	// Run Analysis
	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// We expect main -> path/utils.Helper
	// NOT main -> utils.Helper

	// IDs:
	// utils.Helper
	// path/utils.Helper

	foundCorrect := false
	foundIncorrect := false

	for _, edge := range cg.Edges {
		if edge.From == "main.main" {
			if edge.To == "path/utils.Helper" {
				foundCorrect = true
			}
			if edge.To == "utils.Helper" {
				foundIncorrect = true
			}
		}
	}

	assert.True(t, foundCorrect, "Should resolve to path/utils.Helper (longest suffix match)")
	assert.False(t, foundIncorrect, "Should NOT resolve to utils.Helper (shorter suffix match)")
}
