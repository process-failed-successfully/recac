package analysis

import (
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
	// Setup tmp dir
	tmpDir := t.TempDir()

	// 1. Create main.go
	mainContent := `package main
import (
    "recac/pkg/foo"
)
func main() {
    foo.Do()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// 2. Create pkg/foo/foo.go (longer path)
	err = os.MkdirAll(filepath.Join(tmpDir, "pkg", "foo"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "pkg", "foo", "foo.go"), []byte(`package foo
func Do() {}
`), 0644)
	require.NoError(t, err)

	// 3. Create foo/foo.go (shorter path, also package foo)
	err = os.MkdirAll(filepath.Join(tmpDir, "foo"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tmpDir, "foo", "foo.go"), []byte(`package foo
func Do() {}
`), 0644)
	require.NoError(t, err)

	// Generate Graph
	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Identify the call from main
	var edge *CallGraphEdge
	for _, e := range cg.Edges {
		if strings.Contains(e.From, "main") && strings.HasSuffix(e.To, ".Do") {
			v := e // copy loop var
			edge = &v
			break
		}
	}
	require.NotNil(t, edge, "Should find edge from main to foo.Do")

	// The import is "recac/pkg/foo".
	// "pkg/foo" (pkg.foo) matches.
	// "foo" (foo) matches suffix too if we just check suffix "foo".

	// "recac/pkg/foo" ends with "pkg/foo".
	// "recac/pkg/foo" ends with "foo".

	// Since we prioritize longer package match, it should pick "pkg/foo.Do".

	assert.Equal(t, "pkg/foo.Do", edge.To)
}
