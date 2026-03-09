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

func TestCallGraph_Filter(t *testing.T) {
	// 1. Setup Temp Dir with Go Code
	tmpDir := t.TempDir()

	files := map[string]string{
		"main.go": `package main
func main() {
	Helper()
	Other()
}
func Helper() {
	SubHelper()
}
func SubHelper() {}
func Other() {}
`,
	}
	for name, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	// 2. Setup Command
	cmd := &cobra.Command{
		Use: "callgraph",
		RunE: runCallGraph,
	}
	var buf bytes.Buffer
	cmd.SetOut(&buf)

	// Set Flags
	oldDir := callGraphDir
	oldFocus := callGraphFocus
	defer func() {
		callGraphDir = oldDir
		callGraphFocus = oldFocus
	}()
	callGraphDir = tmpDir
	callGraphFocus = "Helper" // Should include main -> Helper -> SubHelper

	// 3. Run Command
	err := runCallGraph(cmd, []string{})
	require.NoError(t, err)

	output := buf.String()

	// Should contain Helper and its neighbors (main, SubHelper)
	assert.Contains(t, output, "Helper")
	assert.Contains(t, output, "main")
	assert.Contains(t, output, "SubHelper")

	// Should NOT contain Other (not connected to Helper directly or via 1 hop?
	// filterGraph logic expands neighbors of relevant nodes.
	// "Helper" is relevant.
	// "main" (calls Helper) and "SubHelper" (called by Helper) are neighbors, so included.
	// "Other" is called by "main", but "main" is not relevant (just a neighbor),
	// so edges from "main" to non-relevant nodes like "Other" are excluded.

	assert.NotContains(t, output, "Other")
}

func TestCallGraph_InvalidDir(t *testing.T) {
	cmd := &cobra.Command{
		Use: "callgraph",
		RunE: runCallGraph,
	}

	oldDir := callGraphDir
	defer func() { callGraphDir = oldDir }()
	callGraphDir = "/path/to/non/existent/dir"

	err := runCallGraph(cmd, []string{})
	assert.Error(t, err)
}
