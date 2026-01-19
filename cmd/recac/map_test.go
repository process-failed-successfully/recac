package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create an isolated map command for testing
func getTestMapCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "map [path]",
		RunE:  runMap,
	}
	initMapFlags(cmd)
	return cmd
}

func TestMapCmd(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-map-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create go.mod with comment to test parser robustness
	err = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/myproject // My Cool Project\n\ngo 1.20\n"), 0644)
	require.NoError(t, err)

	// Create directories
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "pkg", "util"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "pkg", "api"), 0755))

	// Create main.go
	mainContent := `package main

import (
	"fmt"
	"example.com/myproject/pkg/util"
	"example.com/myproject/pkg/api"
)

func main() {
	fmt.Println(util.Helper())
	api.Serve()
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(mainContent), 0644)
	require.NoError(t, err)

	// Create pkg/util/util.go
	utilContent := `package util

import "fmt"

func Helper() string {
	return fmt.Sprintf("helper")
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "pkg", "util", "util.go"), []byte(utilContent), 0644)
	require.NoError(t, err)

	// Create pkg/api/api.go
	apiContent := `package api

import (
	"example.com/myproject/pkg/util"
)

func Serve() {
	util.Helper()
}
`
	err = os.WriteFile(filepath.Join(tmpDir, "pkg", "api", "api.go"), []byte(apiContent), 0644)
	require.NoError(t, err)

	// Test Mermaid Output
	t.Run("Mermaid Output", func(t *testing.T) {
		cmd := getTestMapCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		output, err := executeCommand(root, "map", tmpDir, "--format", "mermaid")
		require.NoError(t, err)

		assert.Contains(t, output, "graph TD")
		assert.Contains(t, output, "example_com_myproject --> example_com_myproject_pkg_util")
		assert.Contains(t, output, "example_com_myproject --> example_com_myproject_pkg_api")
		assert.Contains(t, output, "example_com_myproject_pkg_api --> example_com_myproject_pkg_util")

		// External deps (fmt) should use dotted line if enabled, or not appear if stdlib disabled (default)
		// Default is --stdlib=false
		assert.NotContains(t, output, "fmt")
	})

	// Test DOT Output
	t.Run("DOT Output", func(t *testing.T) {
		cmd := getTestMapCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		output, err := executeCommand(root, "map", tmpDir, "--format", "dot")
		require.NoError(t, err)

		assert.Contains(t, output, "digraph G {")
		assert.Contains(t, output, "\"example.com/myproject\" -> \"example.com/myproject/pkg/util\";")
		assert.Contains(t, output, "\"example.com/myproject\" -> \"example.com/myproject/pkg/api\";")
	})

	// Test StdLib Flag
	t.Run("StdLib Flag", func(t *testing.T) {
		cmd := getTestMapCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		output, err := executeCommand(root, "map", tmpDir, "--stdlib")
		require.NoError(t, err)

		assert.Contains(t, output, "example_com_myproject -.-> fmt")
	})

	// Test Ignore Flag
	t.Run("Ignore Flag", func(t *testing.T) {
		cmd := getTestMapCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		output, err := executeCommand(root, "map", tmpDir, "--ignore", "pkg/util")
		require.NoError(t, err)

		assert.NotContains(t, output, "example_com_myproject_pkg_util")
		// Main should still depend on API, but not util
		assert.Contains(t, output, "example_com_myproject --> example_com_myproject_pkg_api")
	})

	// Test Focus Flag
	t.Run("Focus Flag", func(t *testing.T) {
		cmd := getTestMapCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		output, err := executeCommand(root, "map", tmpDir, "--focus", "api")
		require.NoError(t, err)

		// Should show API
		assert.Contains(t, output, "example_com_myproject_pkg_api")
		// Should NOT show Main node definition (as source) because it doesn't match "api"
		assert.NotContains(t, output, "example_com_myproject -->")

		// API source should be shown
		assert.Contains(t, output, "example_com_myproject_pkg_api -->")
	})

	// Test Focus Flag with DOT
	t.Run("Focus Flag DOT", func(t *testing.T) {
		cmd := getTestMapCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		output, err := executeCommand(root, "map", tmpDir, "--format", "dot", "--focus", "api")
		require.NoError(t, err)

		assert.Contains(t, output, "\"example.com/myproject/pkg/api\" ->")
		assert.NotContains(t, output, "\"example.com/myproject\" ->")
	})

	// Test Invalid Regex
	t.Run("Invalid Regex", func(t *testing.T) {
		cmd := getTestMapCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		_, err := executeCommand(root, "map", tmpDir, "--ignore", "[")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "invalid ignore pattern")
	})
}
