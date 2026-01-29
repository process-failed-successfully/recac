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

func TestGenerateCallGraph_Determinism(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// Create files
	mainContent := `package main
import "recac-test/pkg"
func main() { pkg.A(); pkg.B() }`
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)

	pkgDir := filepath.Join(tmpDir, "pkg")
	os.MkdirAll(pkgDir, 0755)

	pkgContent := `package pkg
func A() {}
func B() {}`
	os.WriteFile(filepath.Join(pkgDir, "lib.go"), []byte(pkgContent), 0644)

	// Run multiple times and compare
	cg1, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	for i := 0; i < 5; i++ {
		cg2, err := GenerateCallGraph(tmpDir)
		require.NoError(t, err)

		// Compare Edges order
		assert.Equal(t, cg1.Edges, cg2.Edges, "Edges should be deterministic")
	}
}

func TestResolveExternalCall_Ambiguity(t *testing.T) {
	// Verify that if we have "pkg/a" and "pkg/b", and import is "pkg/a", we resolve correctly.
	// But since we can't easily mock the internal state without exporting,
	// we will rely on integration test structure.

	tmpDir := t.TempDir()

	// root/main.go imports "root/sub/util"
	mainContent := `package main
import (
	"fmt"
	"recac-test/sub/util"
)
func main() {
	util.DoSomething()
}
`
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)

	// root/sub/util/util.go
	subDir := filepath.Join(tmpDir, "sub", "util")
	os.MkdirAll(subDir, 0755)
	utilContent := `package util
func DoSomething() {}`
	os.WriteFile(filepath.Join(subDir, "util.go"), []byte(utilContent), 0644)

	// root/util/util.go (Ambiguous name match if we only matched "util")
	// If the logic is "ends with package name", "recac-test/sub/util" ends with "util" (root/util) AND "sub/util" (root/sub/util).
	// But "recac-test/sub/util" vs "util" (at root) -> "recac-test/sub/util" ends with "util"? YES.
	// "recac-test/sub/util" vs "sub/util" -> YES.
	// We want to ensure it picks the best match "sub/util".

	badDir := filepath.Join(tmpDir, "util")
	os.MkdirAll(badDir, 0755)
	badContent := `package util
func DoSomething() {}`
	os.WriteFile(filepath.Join(badDir, "util.go"), []byte(badContent), 0644)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Check where main points to.
	// Expected: main.main -> sub/util.DoSomething
	// If it pointed to root/util, ID would be "util.DoSomething" (or similar depending on rel path)

	// IDs:
	// main.go -> "main.main"
	// sub/util/util.go -> "sub/util.DoSomething"
	// util/util.go -> "util.DoSomething"

	var target string
	for _, edge := range cg.Edges {
		if edge.From == "main.main" {
			target = edge.To
		}
	}

	// We expect "sub/util.DoSomething" because "recac-test/sub/util" ends with "sub/util" (stronger match than just "util"?)
	// Actually, "recac-test/sub/util" DOES NOT end with "util" (the package path from root).
	// Wait, "util" package at root has path "util".
	// "sub/util" package has path "sub/util".
	// Import is "recac-test/sub/util".
	// "recac-test/sub/util" ends with "sub/util".
	// "recac-test/sub/util" ends with "util".

	// Ideally we want strict suffix match or longest match.
	// If we use forward slashes, "sub/util" is the ID.
	// On Windows, it might be "sub\util".

	// Since we haven't fixed ToSlash yet, we should write the test to expect what we WANT (forward slashes).
	// But currently it produces OS specific.
	// Let's assume we want "sub/util.DoSomething".

	// WARNING: If this test runs on Windows before fix, it expects backslashes potentially?
	// But we are running on Linux container usually.

	assert.Equal(t, "sub/util.DoSomething", target, "Should resolve to the correct specific package")
}
