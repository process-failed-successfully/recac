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

func TestResolveExternalCall_Ambiguity(t *testing.T) {
	// Setup:
	// - pkg/utils
	// - internal/pkg/utils
	// - main.go imports "github.com/repo/internal/pkg/utils" (should match internal/pkg/utils)
	// - main.go imports "github.com/repo/pkg/utils" (should match pkg/utils)

	tmpDir := t.TempDir()

	// 1. pkg/utils/util.go
	utilsDir := filepath.Join(tmpDir, "pkg", "utils")
	require.NoError(t, os.MkdirAll(utilsDir, 0755))
	os.WriteFile(filepath.Join(utilsDir, "util.go"), []byte("package utils\nfunc Do() {}"), 0644)

	// 2. internal/pkg/utils/util.go
	intUtilsDir := filepath.Join(tmpDir, "internal", "pkg", "utils")
	require.NoError(t, os.MkdirAll(intUtilsDir, 0755))
	os.WriteFile(filepath.Join(intUtilsDir, "util.go"), []byte("package utils\nfunc Do() {}"), 0644)

	// 3. main.go
	mainContent := `package main
import (
	u1 "github.com/repo/pkg/utils"
	u2 "github.com/repo/internal/pkg/utils"
)
func main() {
	u1.Do()
	u2.Do()
}
`
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Identify nodes
	// pkg/utils.Do
	// internal/pkg/utils.Do

	// Check edges
	var foundU1, foundU2 bool
	for _, edge := range cg.Edges {
		if edge.From == "main.main" {
			if edge.To == "pkg/utils.Do" {
				foundU1 = true
			}
			if edge.To == "internal/pkg/utils.Do" {
				foundU2 = true
			}
		}
	}
	assert.True(t, foundU1, "Should resolve u1 to pkg/utils.Do")
	assert.True(t, foundU2, "Should resolve u2 to internal/pkg/utils.Do")
}

func TestGenerateCallGraph_Determinism(t *testing.T) {
    // Generate graph multiple times and compare edges order
    tmpDir := t.TempDir()
    // Create multiple files
    for i := 0; i < 10; i++ {
        name := filepath.Join(tmpDir, string(rune('a'+i))+".go")
        content := "package main\nfunc F" + string(rune('a'+i)) + "() { F" + string(rune('a'+(i+1)%10)) + "() }"
        os.WriteFile(name, []byte(content), 0644)
    }

    cg1, err := GenerateCallGraph(tmpDir)
    require.NoError(t, err)

    cg2, err := GenerateCallGraph(tmpDir)
    require.NoError(t, err)

    require.Equal(t, len(cg1.Edges), len(cg2.Edges))
    for i := range cg1.Edges {
        assert.Equal(t, cg1.Edges[i], cg2.Edges[i], "Edges should be identical and in same order")
    }
}

func TestGenerateCallGraph_Generics(t *testing.T) {
    // Test multi-parameter generics
    tmpDir := t.TempDir()

    content := `package main

    type Map[K comparable, V any] struct {}

    func (m *Map[K, V]) Get(k K) V {
        var v V
        return v
    }

    func main() {
        m := &Map[string, int]{}
        m.Get("foo")
    }
    `
    os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644)

    cg, err := GenerateCallGraph(tmpDir)
    require.NoError(t, err)

    // Check node ID
    // main.(Map).Get

    foundNode := false
    for id := range cg.Nodes {
        if id == "main.(Map).Get" {
            foundNode = true
        }
    }
    assert.True(t, foundNode, "Should find method on generic type Map")
}
