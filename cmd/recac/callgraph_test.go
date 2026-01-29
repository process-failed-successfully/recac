package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallGraphCmd(t *testing.T) {
	// 1. Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// Create main.go
	// It calls pkg.Helper()
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

	// Create pkg/helper.go
	pkgDir := filepath.Join(tmpDir, "pkg")
	err = os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)

	// Helper calls s.DoWork(), s is Service.
	pkgContent := `package pkg

type Service struct{}

func (s *Service) DoWork() {
}

func Helper() {
	s := &Service{}
	s.DoWork()
}
`
	err = os.WriteFile(filepath.Join(pkgDir, "helper.go"), []byte(pkgContent), 0644)
	require.NoError(t, err)

	// 2. Run callgraph command
	// We need to pass --dir
	output, err := executeCommand(rootCmd, "callgraph", "--dir", tmpDir)
	require.NoError(t, err)

	// 3. Verify Output
	assert.Contains(t, output, "graph LR")

	// Check Nodes
	// main.main -> sanitized main_main or similar (relative path handling)
	// Since we pass tmpDir as root, rel path is ".".
	// main package -> "main". main function -> "main". ID: "main.main"
	// Sanitize("main.main") -> "main_main"
	assert.Contains(t, output, `main_main["main.main"]`)

	// pkg.Helper -> pkg/helper.go -> pkg (dir). ID: "pkg.Helper"
	// Sanitize("pkg.Helper") -> "pkg_Helper"
	assert.Contains(t, output, `pkg_Helper["pkg.Helper"]`)

	// pkg.(Service).DoWork -> ID: "pkg.(Service).DoWork"
	// Sanitize("pkg.(Service).DoWork") -> "pkg__Service__DoWork"
	assert.Contains(t, output, `pkg__Service__DoWork["pkg.(Service).DoWork"]`)

	// Check Edges
	// main.main --> pkg.Helper
	assert.Contains(t, output, `main_main --> pkg_Helper`)

	// pkg.Helper --> pkg.(Service).DoWork
	assert.Contains(t, output, `pkg_Helper --> pkg__Service__DoWork`)
}

func TestCallGraphCmd_Focus(t *testing.T) {
	// 1. Setup temporary directory with sample code
	tmpDir := t.TempDir()

	mainContent := `package main
func A() { B() }
func B() { C() }
func C() {}
func D() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// 2. Run with focus "B"
	output, err := executeCommand(rootCmd, "callgraph", "--dir", tmpDir, "--focus", "B")
	require.NoError(t, err)

	// 3. Verify
	// Should contain A (caller), B (focus), C (callee)
	// Should NOT contain D
	assert.Contains(t, output, `main_A`)
	assert.Contains(t, output, `main_B`)
	assert.Contains(t, output, `main_C`)
	assert.NotContains(t, output, `main_D`)
}

func TestCallGraphCmd_FocusEmpty(t *testing.T) {
	// 1. Setup temporary directory
	tmpDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main(){}"), 0644)
	require.NoError(t, err)

	// 2. Run with non-existent focus
	output, err := executeCommand(rootCmd, "callgraph", "--dir", tmpDir, "--focus", "NonExistent")
	require.NoError(t, err)

	// 3. Verify Empty Graph
	// Should verify that it does NOT contain "main"
	assert.NotContains(t, output, "main_main")
	// Should still have "graph LR" probably?
	assert.Contains(t, output, "graph LR")
}

func TestCallGraphCmd_Sanitization(t *testing.T) {
	// Verify that complex signatures are sanitized
	// Test case where ID has special chars
	// This implicitly tests sanitizeMermaidID integration via GenerateCallGraph

	tmpDir := t.TempDir()

	// Struct with Generic-like name or special chars? Go identifiers are limited.
	// But our IDs include package paths.

	// Create a nested directory structure "a-b/c.d"
	nestedDir := filepath.Join(tmpDir, "a-b", "c.d")
	err := os.MkdirAll(nestedDir, 0755)
	require.NoError(t, err)

	content := `package d
func Func() {}
`
	err = os.WriteFile(filepath.Join(nestedDir, "file.go"), []byte(content), 0644)
	require.NoError(t, err)

	// Run
	output, err := executeCommand(rootCmd, "callgraph", "--dir", tmpDir)
	require.NoError(t, err)

	// ID should be "a-b/c.d/d.Func" (approx)
	// Sanitized: "a_b_c_d_d_Func"
	// Check that output DOES NOT contain "a-b" or "c.d" in the NODE ID part (left side of bracket)

	// We expect something like: a_b_c_d_d_Func["d.Func"]
	// Warning: The label logic simplifies to last part.

	// Let's print output if failed
	if !strings.Contains(output, "a_b_c_d") {
		// It might be normalized differently depending on analysis logic
		// internal/analysis/callgraph.go uses relDir.
		// relDir = "a-b/c.d".
		// fullPkg = "a-b/c.d/d" (if pkg name is d)
		// ID = "a-b/c.d/d.Func"
		// Sanitize -> "a_b_c_d_d_Func"

		// Assert that we don't have hyphens in ID
		// Regex check would be better but simple string check:
		// Find line with Func
		lines := strings.Split(output, "\n")
		for _, line := range lines {
			if strings.Contains(line, "Func") && strings.Contains(line, "[") {
				// ID is before [
				parts := strings.Split(line, "[")
				id := strings.TrimSpace(parts[0])
				assert.NotContains(t, id, "-", "Node ID should not contain hyphen")
				assert.NotContains(t, id, ".", "Node ID should not contain dot")
				assert.NotContains(t, id, "/", "Node ID should not contain slash")
			}
		}
	}
}
