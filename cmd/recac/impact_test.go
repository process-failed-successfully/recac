package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper to create an isolated impact command for testing
func getTestImpactCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "impact [files...]",
		RunE:  runImpact,
	}
	// Define flags locally
	cmd.Flags().Bool("json", false, "Output results as JSON")
	cmd.Flags().Bool("suggest-tests", false, "Suggest relevant tests to run")
	cmd.Flags().Bool("git-diff", false, "Analyze changes in current git diff")
	return cmd
}

func TestImpactCmd(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-impact-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Switch to temp dir so "." works as expected
	cwd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(cwd)
	require.NoError(t, os.Chdir(tmpDir))

	// Create go.mod
	err = os.WriteFile("go.mod", []byte("module example.com/impactproj\n\ngo 1.20\n"), 0644)
	require.NoError(t, err)

	// Create pkg/util/util.go (Leaf)
	require.NoError(t, os.MkdirAll(filepath.Join("pkg", "util"), 0755))
	utilContent := `package util
func Helper() string { return "helper" }
`
	err = os.WriteFile(filepath.Join("pkg", "util", "util.go"), []byte(utilContent), 0644)
	require.NoError(t, err)

	// Create pkg/mid/mid.go (Depends on util)
	require.NoError(t, os.MkdirAll(filepath.Join("pkg", "mid"), 0755))
	midContent := `package mid
import "example.com/impactproj/pkg/util"
func Mid() { util.Helper() }
`
	err = os.WriteFile(filepath.Join("pkg", "mid", "mid.go"), []byte(midContent), 0644)
	require.NoError(t, err)

	// Create pkg/api/api.go (Depends on mid)
	require.NoError(t, os.MkdirAll(filepath.Join("pkg", "api"), 0755))
	apiContent := `package api
import "example.com/impactproj/pkg/mid"
func API() { mid.Mid() }
`
	err = os.WriteFile(filepath.Join("pkg", "api", "api.go"), []byte(apiContent), 0644)
	require.NoError(t, err)

	// Create main.go (Depends on api)
	mainContent := `package main
import "example.com/impactproj/pkg/api"
func main() { api.API() }
`
	err = os.WriteFile("main.go", []byte(mainContent), 0644)
	require.NoError(t, err)

	// Add a test file in api
	apiTestContent := `package api
import "testing"
func TestAPI(t *testing.T) { API() }
`
	err = os.WriteFile(filepath.Join("pkg", "api", "api_test.go"), []byte(apiTestContent), 0644)
	require.NoError(t, err)

	// Test 1: Change Leaf (util) -> Should affect util, mid, api, main
	t.Run("Change Leaf Node", func(t *testing.T) {
		cmd := getTestImpactCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		// We are already in tmpDir
		output, err := executeCommand(root, "impact", "pkg/util/util.go")
		require.NoError(t, err)

		assert.Contains(t, output, "example.com/impactproj/pkg/util (changed)")
		assert.Contains(t, output, "example.com/impactproj/pkg/mid")
		assert.Contains(t, output, "example.com/impactproj/pkg/api")
		assert.Contains(t, output, "example.com/impactproj") // main package
	})

	// Test 2: Change Mid -> Should affect mid, api, main (but NOT util)
	t.Run("Change Middle Node", func(t *testing.T) {
		cmd := getTestImpactCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		output, err := executeCommand(root, "impact", "pkg/mid/mid.go")
		require.NoError(t, err)

		assert.NotContains(t, output, "pkg/util")
		assert.Contains(t, output, "example.com/impactproj/pkg/mid (changed)")
		assert.Contains(t, output, "example.com/impactproj/pkg/api")
		assert.Contains(t, output, "example.com/impactproj")
	})

	// Test 3: Suggest Tests
	t.Run("Suggest Tests", func(t *testing.T) {
		cmd := getTestImpactCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		output, err := executeCommand(root, "impact", "--suggest-tests", "pkg/util/util.go")
		require.NoError(t, err)

		assert.Contains(t, output, "Suggested Tests:")
		// API has a test file
		assert.Contains(t, output, "go test example.com/impactproj/pkg/api")
	})

	// Test 4: JSON Output
	t.Run("JSON Output", func(t *testing.T) {
		cmd := getTestImpactCmd()
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(cmd)

		output, err := executeCommand(root, "impact", "--json", "pkg/util/util.go")
		require.NoError(t, err)

		var res ImpactResult
		err = json.Unmarshal([]byte(output), &res)
		require.NoError(t, err)

		assert.Contains(t, res.AffectedPackages, "example.com/impactproj/pkg/util")
		assert.Contains(t, res.AffectedPackages, "example.com/impactproj/pkg/mid")
		assert.Contains(t, res.AffectedPackages, "example.com/impactproj/pkg/api")
	})
}
