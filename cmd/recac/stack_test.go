package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStackCommand(t *testing.T) {
	// Create a temp directory representing a project
	tmpDir, err := os.MkdirTemp("", "recac-stack-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// create some dummy files
	files := map[string]string{
		"main.go": `package main
import (
	"fmt"
	"github.com/spf13/cobra"
)
func main() {}`,
		"package.json": `{
  "dependencies": {
    "react": "^18.0.0",
    "express": "^4.0.0"
  }
}`,
		"docker-compose.yml": `version: "3"
services:
  db:
    image: postgres:14
  redis:
    image: redis:alpine
`,
		"go.mod": `module example.com/test
require (
	github.com/spf13/cobra v1.7.0
)
`,
		".github/workflows/ci.yml": "name: CI",
	}

	for path, content := range files {
		fullPath := filepath.Join(tmpDir, path)
		err := os.MkdirAll(filepath.Dir(fullPath), 0755)
		require.NoError(t, err)
		err = os.WriteFile(fullPath, []byte(content), 0644)
		require.NoError(t, err)
	}

	t.Run("AnalyzeStack", func(t *testing.T) {
		info, err := analyzeStack(tmpDir)
		require.NoError(t, err)

		// Check Languages
		assert.Equal(t, 1, info.Languages["Go"])
		// JSON isn't a language in our map unless .json extension is handled?
		// looking at code: .json is not in getLanguage.
		// But let's check what we have.

		// Check Frameworks
		assert.Contains(t, info.Frameworks, "Cobra")
		assert.Contains(t, info.Frameworks, "React")
		assert.Contains(t, info.Frameworks, "Express")

		// Check Infra
		assert.Contains(t, info.Infrastructure, "Docker Compose")

		// Check DBs
		assert.Contains(t, info.Databases, "PostgreSQL")
		assert.Contains(t, info.Databases, "Redis")

		// Check CI
		assert.Contains(t, info.CI, "GitHub Actions")
	})

	t.Run("JSON Output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "stack", tmpDir, "--json")
		require.NoError(t, err)

		var info StackInfo
		err = json.Unmarshal([]byte(output), &info)
		require.NoError(t, err)

		assert.Equal(t, 1, info.Languages["Go"])
		assert.Contains(t, info.Frameworks, "React")
	})

	t.Run("Mermaid Output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "stack", tmpDir, "--mermaid")
		require.NoError(t, err)

		assert.Contains(t, output, "graph TD")
		assert.Contains(t, output, "App[Go App]")
		assert.Contains(t, output, "React")
		assert.Contains(t, output, "PostgreSQL")
	})

	t.Run("Table Output", func(t *testing.T) {
		output, err := executeCommand(rootCmd, "stack", tmpDir)
		require.NoError(t, err)

		assert.Contains(t, output, "PROJECT STACK")
		assert.Contains(t, output, "Go (1 files)")
		assert.Contains(t, output, "React")
		assert.Contains(t, output, "PostgreSQL")
	})
}
