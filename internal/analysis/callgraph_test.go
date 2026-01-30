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
	// Mock CodebaseIndex with manually populated PkgIndex
	idx := &CodebaseIndex{
		PkgIndex: map[string][]*CallGraphNode{
			"pkg":        {{ID: "pkg.Func", Name: "Func"}},
			"other/pkg":  {{ID: "other/pkg.Func", Name: "Func"}},
			"mypkg":      {{ID: "mypkg.Func", Name: "Func"}},
		},
	}

	tests := []struct {
		name       string
		importPath string
		funcName   string
		wantID     string
	}{
		{
			name:       "Exact match",
			importPath: "pkg",
			funcName:   "Func",
			wantID:     "pkg.Func",
		},
		{
			name:       "Suffix match with slash",
			importPath: "github.com/user/pkg",
			funcName:   "Func",
			wantID:     "pkg.Func",
		},
		{
			name:       "Suffix match nested",
			importPath: "github.com/user/other/pkg",
			funcName:   "Func",
			wantID:     "other/pkg.Func",
		},
		{
			name:       "Partial suffix match should FAIL",
			importPath: "github.com/user/mypkg",
			funcName:   "Func",
			wantID:     "mypkg.Func", // Matches mypkg exactly
		},
		{
			name:       "Ambiguous partial suffix check",
			// "pkg" is in index. "mypkg" ends with "pkg".
			// But we expect it NOT to match "pkg".
			importPath: "mypkg",
			funcName:   "Func",
			wantID:     "mypkg.Func", // Matches "mypkg" exactly.
		},
		{
			name:       "Strict suffix boundary check",
			// "github.com/somepkg" vs "pkg"
			// "somepkg" ends with "pkg" but no slash.
			// Should NOT match "pkg".
			importPath: "github.com/somepkg",
			funcName:   "Func",
			wantID:     "",
		},
		{
			name:       "Short exact match vs Long suffix match",
			// If we have "foo/bar" and "bar".
			// Import "foo/bar".
			// Should match "foo/bar", not "bar".
			importPath: "foo/bar",
			funcName:   "Func",
			wantID:     "", // We didn't put foo/bar in index, so empty.
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveExternalCall(idx, tt.importPath, tt.funcName)
			assert.Equal(t, tt.wantID, got)
		})
	}
}
