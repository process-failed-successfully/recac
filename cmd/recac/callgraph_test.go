package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallGraphCmd(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// 1. Create main.go
	mainContent := `package main
func main() {
	Helper()
}
func Helper() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Initialize Command
	cmd := NewCallGraphCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--dir", tmpDir})

	// Execute
	err = cmd.Execute()
	require.NoError(t, err)

	// Verify Output
	output := buf.String()
	assert.Contains(t, output, "graph LR")

	// IDs should be sanitized. main.main -> main_main. main.Helper -> main_Helper
	// Note: internal/analysis resolves package paths.
	// Since we are running in a tmp dir which is not in GOPATH or module, it might use "main" as package.

	// Check for presence of node definitions
	assert.Contains(t, output, "main_main")
	assert.Contains(t, output, "main_Helper")

	// Check for edge
	assert.Contains(t, output, "main_main --> main_Helper")
}

func TestCallGraphCmd_Focus(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// 1. Create main.go
	mainContent := `package main
func main() {
	Helper()
	Other()
}
func Helper() {}
func Other() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Initialize Command with Focus
	cmd := NewCallGraphCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--dir", tmpDir, "--focus", "Helper"})

	// Execute
	err = cmd.Execute()
	require.NoError(t, err)

	// Verify Output
	output := buf.String()
	assert.Contains(t, output, "graph LR")
	assert.Contains(t, output, "main_Helper")

	// Should show caller main
	assert.Contains(t, output, "main_main")

	// Should NOT show Other (unconnected to Helper)
	// We check for the label or ID presentation in Mermaid
	assert.NotContains(t, output, "main_Other[\"Other\"]")
}

func TestCallGraphCmd_Sanitization(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// 1. Create pkg/file.go with method
	pkgDir := filepath.Join(tmpDir, "pkg")
	err := os.MkdirAll(pkgDir, 0755)
	require.NoError(t, err)

	content := `package pkg
type MyType struct{}
func (m *MyType) MyMethod() {}
`
	err = os.WriteFile(filepath.Join(pkgDir, "file.go"), []byte(content), 0644)
	require.NoError(t, err)

	// Initialize Command
	cmd := NewCallGraphCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--dir", tmpDir})

	// Execute
	err = cmd.Execute()
	require.NoError(t, err)

	output := buf.String()

	// ID should be pkg.(MyType).MyMethod
	// Sanitized: pkg__MyType__MyMethod
	// Parenthesis should be replaced by underscores

	assert.Contains(t, output, "pkg__MyType__MyMethod")
	// Label usually includes package/type info if present
	assert.Contains(t, output, "[\"pkg.(MyType).MyMethod\"]")
}
