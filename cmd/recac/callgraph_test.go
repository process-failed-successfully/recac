package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallGraphCmd(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// Create main.go
	mainContent := `package main
func main() {
    foo()
}
func foo() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Run command
	// We use the executeCommand helper defined in test_helpers_test.go
	// which mocks stdout/stderr and captures output.
	output, err := executeCommand(rootCmd, "callgraph", "--dir", tmpDir)
	require.NoError(t, err)

	// Verify Output
	assert.Contains(t, output, "graph LR")
	// The nodes should be sanitized: "main.main" -> "main_main", "main.foo" -> "main_foo"
	// Note: IDs are constructed as "relPath/Package.Func".
	// Since we use t.TempDir(), path might be complex.
	// But analysis.GenerateCallGraph resolves relative to root.
	// If we pass tmpDir as root, relPath is ".".
	// So package is "main" (folder name) or "main" (package name if rel is .).
	// Let's check logic in analysis/callgraph.go:
	// if relDir == "." { fullPkg = pkgName }
	// So fullPkg is "main".
	// ID is "main.main".
	// Sanitized ID is "main_main".

	assert.Contains(t, output, `main_main["main.main"]`)
	assert.Contains(t, output, `main_foo["main.foo"]`)
	assert.Contains(t, output, "main_main --> main_foo")
}

func TestCallGraphCmd_Focus(t *testing.T) {
	tmpDir := t.TempDir()

	mainContent := `package main
func main() {
    foo()
	bar()
}
func foo() {}
func bar() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Run with focus on "foo"
	output, err := executeCommand(rootCmd, "callgraph", "--dir", tmpDir, "--focus", "foo")
	require.NoError(t, err)

	assert.Contains(t, output, "graph LR")
	assert.Contains(t, output, "main_foo")
	assert.Contains(t, output, "main_main") // Caller of foo

	// bar should NOT be in the output as it is not connected to foo directly (it's a sibling callee of main)
	// Wait, filterGraph expands to 1 level.
	// relevantNodes: foo.
	// Edges: main->foo.
	// relevantNodes: foo, main.
	// Edges included: main->foo.
	// Does it include main->bar?
	// Edges loop:
	// main->foo: relevantNodes[main] is true (after expansion). YES.
	// main->bar: relevantNodes[main] is true. YES.

	// Wait, let's look at filterGraph logic again.
	/*
	expandedNodes := make(map[string]bool)
	for id := range relevantNodes {
		expandedNodes[id] = true
	}

	for _, edge := range cg.Edges {
		if relevantNodes[edge.From] || relevantNodes[edge.To] {
			filteredEdges = append(filteredEdges, edge)
			expandedNodes[edge.From] = true
			expandedNodes[edge.To] = true
		}
	}
	*/

	// 1. relevantNodes = {foo}
	// 2. expandedNodes = {foo}
	// 3. Edges:
	//    main->foo. relevantNodes[foo] is true. -> Include edge. expandedNodes += main.
	//    main->bar. relevantNodes[main] is false (at check time? No, relevantNodes is map).
	//               relevantNodes[bar] is false.
	//               -> Skip edge.

	// So bar should NOT be included.
	assert.NotContains(t, output, "main_bar")
}
