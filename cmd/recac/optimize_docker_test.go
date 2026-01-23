package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestOptimizeDockerCmd(t *testing.T) {
	tempDir := t.TempDir()
	dockerfile := filepath.Join(tempDir, "Dockerfile")

	content := `
FROM node:latest
RUN npm install
RUN npm run build
`
	if err := os.WriteFile(dockerfile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	t.Run("Basic Analysis", func(t *testing.T) {
		out, err := executeCommand(rootCmd, "optimize-docker", "--file", dockerfile)
		assert.NoError(t, err)
		assert.Contains(t, out, "explicit_tag")
		assert.Contains(t, out, "combine_run")
		assert.Contains(t, out, "user_check")
	})

	t.Run("Ignore Rule", func(t *testing.T) {
		out, err := executeCommand(rootCmd, "optimize-docker", "--file", dockerfile, "--ignore", "explicit_tag,user_check")
		assert.NoError(t, err)
		assert.NotContains(t, out, "explicit_tag")
		assert.NotContains(t, out, "user_check")
		assert.Contains(t, out, "combine_run")
	})

	t.Run("JSON Output", func(t *testing.T) {
		out, err := executeCommand(rootCmd, "optimize-docker", "--file", dockerfile, "--json")
		assert.NoError(t, err)
		assert.Contains(t, out, `"rule": "explicit_tag"`)
	})
}
