package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSBOMCommand(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir, err := os.MkdirTemp("", "recac-sbom-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Change cwd to tmpDir because sbom command works on current dir
	cwd, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(cwd)
	err = os.Chdir(tmpDir)
	require.NoError(t, err)

	// 2. Create sample files
	files := map[string]string{
		"go.mod": `module example.com/myproject

go 1.21

require (
	github.com/spf13/cobra v1.8.0
	github.com/stretchr/testify v1.8.4
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
)
`,
		"package.json": `{
  "name": "my-frontend",
  "version": "1.0.0",
  "dependencies": {
    "react": "^18.2.0",
    "axios": "~1.5.0"
  },
  "devDependencies": {
    "typescript": "5.2.2"
  }
}`,
	}

	for path, content := range files {
		err := os.WriteFile(filepath.Join(tmpDir, path), []byte(content), 0644)
		require.NoError(t, err)
	}

	t.Run("Default (SPDX)", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "sbom")
		require.NoError(t, err)

		var doc SPDXDocument
		err = json.Unmarshal([]byte(output), &doc)
		require.NoError(t, err)

		assert.Equal(t, "SPDX-2.3", doc.SPDXVersion)
		// Check packages
		foundCobra := false
		foundReact := false
		foundTS := false

		for _, p := range doc.Packages {
			if p.Name == "github.com/spf13/cobra" {
				foundCobra = true
				assert.Equal(t, "v1.8.0", p.VersionInfo)
			}
			if p.Name == "react" {
				foundReact = true
				assert.Equal(t, "18.2.0", p.VersionInfo) // cleaned version
			}
			if p.Name == "typescript" {
				foundTS = true
				assert.Equal(t, "5.2.2", p.VersionInfo)
			}
		}
		assert.True(t, foundCobra, "Cobra not found in SBOM")
		assert.True(t, foundReact, "React not found in SBOM")
		assert.True(t, foundTS, "Typescript not found in SBOM")
	})

	t.Run("CycloneDX", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "sbom", "--format", "cyclonedx")
		require.NoError(t, err)

		var doc CycloneDXDocument
		err = json.Unmarshal([]byte(output), &doc)
		require.NoError(t, err)

		assert.Equal(t, "CycloneDX", doc.BomFormat)

		foundAxios := false
		for _, c := range doc.Components {
			if c.Name == "axios" {
				foundAxios = true
				assert.Equal(t, "1.5.0", c.Version)
				assert.Equal(t, "pkg:npm/axios@1.5.0", c.Purl)
			}
		}
		assert.True(t, foundAxios, "Axios not found in CycloneDX SBOM")
	})

	t.Run("Simple JSON", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "sbom", "--format", "json")
		require.NoError(t, err)

		// Should be a list of Package
		var pkgs []map[string]interface{}
		err = json.Unmarshal([]byte(output), &pkgs)
		require.NoError(t, err)

		assert.NotEmpty(t, pkgs)
	})

	t.Run("Output to File", func(t *testing.T) {
		outFile := filepath.Join(tmpDir, "sbom.json")
		_, err := executeCommand(rootCmd, "sbom", "-o", outFile)
		require.NoError(t, err)

		content, err := os.ReadFile(outFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "SPDX-2.3")
	})
}
