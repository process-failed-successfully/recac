package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRunCallGraph(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a simple go file
	src := `package main
func main() {
	foo()
}
func foo() {
	bar()
}
func bar() {}
`
	err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(src), 0644)
	require.NoError(t, err)

	// Run callgraph
	// callGraphCmd flags: --dir, --focus

	callGraphCmd.Flags().Set("dir", tmpDir)
	callGraphCmd.Flags().Set("focus", "")

	b := new(bytes.Buffer)
	callGraphCmd.SetOut(b)
	callGraphCmd.SetErr(b)

	err = runCallGraph(callGraphCmd, []string{})
	require.NoError(t, err)

	output := b.String()
	// Output should be Mermaid graph
	// Depending on analysis implementation, IDs might be fully qualified.
	// But generateMermaidCallGraph simplifies labels.
	// IDs: "main.main", "main.foo".

	assert.Contains(t, output, "graph LR")
	// We check for edges roughly
	// sanitizeMermaidID replaces dots with underscores likely.
	// So "main_main --> main_foo" or similar.
	// But label shows "main" or "foo".

	// Let's check labels appear
	assert.Contains(t, output, "foo")
	assert.Contains(t, output, "bar")

	// Check edge arrow
	assert.Contains(t, output, "-->")
}

func TestRunCallGraph_Focus(t *testing.T) {
	tmpDir := t.TempDir()

	src := `package main
func main() {
	foo()
}
func foo() {
	bar()
}
func bar() {}
`
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(src), 0644)

	callGraphCmd.Flags().Set("dir", tmpDir)
	callGraphCmd.Flags().Set("focus", "foo")

	b := new(bytes.Buffer)
	callGraphCmd.SetOut(b)
	callGraphCmd.SetErr(b)

	err := runCallGraph(callGraphCmd, []string{})
	require.NoError(t, err)

	output := b.String()
	assert.Contains(t, output, "graph LR")

	// Focus on foo should keep main->foo and foo->bar
	assert.Contains(t, output, "foo")
	assert.Contains(t, output, "bar")
	assert.Contains(t, output, "-->")
}
