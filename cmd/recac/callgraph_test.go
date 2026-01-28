package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCallGraphCommand(t *testing.T) {
	// Setup temporary directory with sample code
	tmpDir := t.TempDir()

	// 1. Create main.go
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

	pkgContent := `package pkg

func Helper() {
	internalFunc()
}

func internalFunc() {}
`
	err = os.WriteFile(filepath.Join(pkgDir, "helper.go"), []byte(pkgContent), 0644)
	require.NoError(t, err)

	// Helper function to execute command
	execute := func(args []string) (string, error) {
		buf := new(bytes.Buffer)

		// Reset global flags
		callGraphDir = "."
		callGraphFocus = ""

		// Use rootCmd to ensure proper command routing and flag parsing
		cmd := rootCmd
		cmd.SetOut(buf)
		cmd.SetErr(buf)

		// Prepend subcommand name
		fullArgs := append([]string{"callgraph"}, args...)
		cmd.SetArgs(fullArgs)

		err := cmd.Execute()
		return buf.String(), err
	}

	t.Run("Generate full graph", func(t *testing.T) {
		output, err := execute([]string{"--dir", tmpDir})
		require.NoError(t, err)

		assert.Contains(t, output, "graph LR")
		assert.Contains(t, output, "main.main")
		assert.Contains(t, output, "pkg.Helper")
		assert.Contains(t, output, "pkg.internalFunc")
		// Edges use sanitized IDs (replaced dots with underscores)
		assert.Contains(t, output, "main_main --> pkg_Helper")
	})

	t.Run("Focus on specific function", func(t *testing.T) {
		output, err := execute([]string{"--dir", tmpDir, "--focus", "Helper"})
		require.NoError(t, err)

		assert.Contains(t, output, "graph LR")
		assert.Contains(t, output, "pkg.Helper")
		// Focused graph should include connected nodes
		assert.Contains(t, output, "main.main") // caller
		assert.Contains(t, output, "pkg.internalFunc") // callee
	})

	t.Run("Focus with no match", func(t *testing.T) {
		// Run callgraph --dir tmpDir --focus NonExistent
		output, err := execute([]string{"--dir", tmpDir, "--focus", "NonExistent"})
		require.NoError(t, err)

		// Logic: if len(relevantNodes) == 0 { return cg }
		// So it returns the FULL graph if nothing matches
		assert.Contains(t, output, "graph LR")
		assert.Contains(t, output, "main.main")
	})

	t.Run("Error on non-existent directory", func(t *testing.T) {
		_, err := execute([]string{"--dir", "/path/to/nowhere"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to generate call graph")
	})
}
