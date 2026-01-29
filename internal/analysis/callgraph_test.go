package analysis

import (
	"fmt"
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

	// Create multiple files to trigger random iteration order if not sorted.
	for i := 0; i < 20; i++ {
		fileName := fmt.Sprintf("file%d.go", i)
		content := fmt.Sprintf(`package pkg
func Func%d() {
	Func%d()
}
`, i, (i+1)%20)
		err := os.WriteFile(filepath.Join(tmpDir, fileName), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Run 1
	cg1, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Run 2
	cg2, err := GenerateCallGraph(tmpDir)
	require.NoError(t, err)

	// Compare Edges Order
	require.Equal(t, len(cg1.Edges), len(cg2.Edges))
	for i := range cg1.Edges {
		assert.Equal(t, cg1.Edges[i], cg2.Edges[i], "Edge at index %d mismatch. Expected %v, got %v", i, cg1.Edges[i], cg2.Edges[i])
	}
}

func TestResolveExternalCall_Ambiguity(t *testing.T) {
	// Setup nodes directly
	cg := &CallGraph{
		Nodes: map[string]*CallGraphNode{
			"pkg/utils.Helper": {
				ID:      "pkg/utils.Helper",
				Package: "pkg/utils",
				Name:    "Helper",
			},
			"internal/pkg/utils.Helper": {
				ID:      "internal/pkg/utils.Helper",
				Package: "internal/pkg/utils",
				Name:    "Helper",
			},
		},
	}

	// Ambiguous call
	// Import: "github.com/repo/internal/pkg/utils"
	// Should match "internal/pkg/utils" better than "pkg/utils"
	// because it's a longer suffix match.
	// Current implementation picks first match (random map order), so this might fail.

	match := resolveExternalCall(cg, "github.com/repo/internal/pkg/utils", "Helper")
	assert.Equal(t, "internal/pkg/utils.Helper", match)
}
