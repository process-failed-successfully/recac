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

func TestGenerateCallGraph_SkipsVendorAndTestdata(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a normal file
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc Main() {}"), 0644)
	require.NoError(t, err)

	// Create vendor directory and file
	vendorDir := filepath.Join(tmpDir, "vendor", "foo")
	err = os.MkdirAll(vendorDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(vendorDir, "foo.go"), []byte("package foo\nfunc VendorFunc() {}"), 0644)
	require.NoError(t, err)

	// Create testdata directory and file
	testdataDir := filepath.Join(tmpDir, "testdata")
	err = os.MkdirAll(testdataDir, 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(testdataDir, "bad.go"), []byte("package bad\nfunc BadFunc() {}"), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	for id := range cg.Nodes {
		assert.NotContains(t, id, "vendor", "Should not contain vendor nodes")
		assert.NotContains(t, id, "testdata", "Should not contain testdata nodes")
		assert.NotEqual(t, "foo.VendorFunc", id) // Simplified check
		assert.NotEqual(t, "bad.BadFunc", id)
	}
}

func TestGenerateCallGraph_Generics(t *testing.T) {
	tmpDir := t.TempDir()

	// Create generic_struct.go
	content := `package pkg

type Container[T any] struct {
	val T
}

func (c *Container[T]) Get() T {
	return c.val
}

// Multi-type param
type Pair[K, V any] struct {
	k K
	v V
}

func (p *Pair[K, V]) GetKey() K {
	return p.k
}

func User() {
	c := &Container[int]{val: 1}
	c.Get()

	p := &Pair[string, int]{k: "key", v: 1}
	p.GetKey()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "generic.go"), []byte(content), 0644)
	require.NoError(t, err)

	cg, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Verify Nodes
	nodeIDs := make(map[string]bool)
	for id := range cg.Nodes {
		nodeIDs[id] = true
	}

	// pkgName="pkg". generic.go in root.
	// relDir=".". fullPkg="pkg".

	assert.Contains(t, nodeIDs, "pkg.(Container).Get")
	assert.Contains(t, nodeIDs, "pkg.(Pair).GetKey")
	assert.Contains(t, nodeIDs, "pkg.User")

	// Verify Edges
	foundUserToGet := false
	foundUserToGetKey := false

	for _, edge := range cg.Edges {
		if edge.From == "pkg.User" && edge.To == "pkg.(Container).Get" {
			foundUserToGet = true
		}
		if edge.From == "pkg.User" && edge.To == "pkg.(Pair).GetKey" {
			foundUserToGetKey = true
		}
	}

	assert.True(t, foundUserToGet, "Missing edge: User -> Container.Get")
	assert.True(t, foundUserToGetKey, "Missing edge: User -> Pair.GetKey")
}
