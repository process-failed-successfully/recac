package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallGraphCmd(t *testing.T) {
	// Setup a temporary directory with some Go code
	tmpDir := t.TempDir()

	mainContent := `package main
func main() {
	foo()
}
func foo() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Save/Restore globals
	oldDir := callGraphDir
	oldFocus := callGraphFocus
	defer func() {
		callGraphDir = oldDir
		callGraphFocus = oldFocus
	}()

	callGraphDir = tmpDir
	callGraphFocus = ""

	// Execute
	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err = runCallGraph(cmd, []string{})
	require.NoError(t, err)

	output := out.String()
	assert.Contains(t, output, "graph LR")

	// Check for sanitized IDs
	// Node IDs should be "main_main" and "main_foo" (approx)
	// They should NOT contain "." or "(" if we had them.
	// Here we have simple names.

	assert.Contains(t, output, "main_main")
	assert.Contains(t, output, "main_foo")
}

func TestCallGraphCmd_ComplexIDs(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package pkg
type S struct{}
func (s *S) Method() {}
func Call() {
	s := &S{}
	s.Method()
}
`
	err := os.WriteFile(filepath.Join(tmpDir, "pkg.go"), []byte(content), 0644)
	require.NoError(t, err)

	oldDir := callGraphDir
	oldFocus := callGraphFocus
	defer func() {
		callGraphDir = oldDir
		callGraphFocus = oldFocus
	}()

	callGraphDir = tmpDir
	callGraphFocus = ""

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err = runCallGraph(cmd, []string{})
	require.NoError(t, err)

	output := out.String()

	// ID for method: pkg.(S).Method
	// Sanitized: pkg__S__Method
	// The ID used in the graph definition (left side of bracket) must be sanitized.

	assert.Contains(t, output, "pkg__S__Method")

	// Verify that we don't have unsanitized IDs on the left side
	// We can't easily regex this without parsing, but we can check specific known bad strings
	// "pkg.(S).Method[" is bad. "pkg__S__Method[" is good.

	assert.Contains(t, output, "pkg__S__Method[\"pkg.(S).Method\"]")
}

func TestCallGraphCmd_Focus(t *testing.T) {
	tmpDir := t.TempDir()

	content := `package main
func A() { B() }
func B() { C() }
func C() {}
func D() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(content), 0644)
	require.NoError(t, err)

	oldDir := callGraphDir
	oldFocus := callGraphFocus
	defer func() {
		callGraphDir = oldDir
		callGraphFocus = oldFocus
	}()

	callGraphDir = tmpDir
	callGraphFocus = "B"

	cmd := &cobra.Command{}
	var out bytes.Buffer
	cmd.SetOut(&out)

	err = runCallGraph(cmd, []string{})
	require.NoError(t, err)

	output := out.String()

	// B calls C, A calls B.
	// Focus B should show A->B and B->C.
	// D should be absent.

	assert.Contains(t, output, "main_A")
	assert.Contains(t, output, "main_B")
	assert.Contains(t, output, "main_C")
	assert.NotContains(t, output, "main_D")
}
