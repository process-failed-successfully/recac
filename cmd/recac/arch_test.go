package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchCmd(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-arch-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	err = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/archtest\n\ngo 1.20\n"), 0644)
	require.NoError(t, err)

	// Create layers: domain, app

	// internal/domain/model.go
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "domain"), 0755))
	domainContent := `package domain
func Model() {}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "domain", "model.go"), []byte(domainContent), 0644))

	// internal/app/service.go -> imports domain
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "app"), 0755))
	appContent := `package app
import "example.com/archtest/internal/domain"
func Service() { domain.Model() }`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "app", "service.go"), []byte(appContent), 0644))

	// Create config file
	configContent := `layers:
  domain: "internal/domain.*"
  app: "internal/app.*"

rules:
  - from: "domain"
    allow: []
  - from: "app"
    allow: ["domain"]
`
	configFile := filepath.Join(tmpDir, ".recac-arch.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	t.Run("Passes Valid Architecture", func(t *testing.T) {
		root := &cobra.Command{Use: "recac"}
		root.AddCommand(archCmd)

		output, err := executeCommand(root, "arch", tmpDir)
		assert.NoError(t, err)
		assert.Contains(t, output, "Architecture check passed")
	})

	t.Run("Fails Invalid Architecture", func(t *testing.T) {
		// Create a violation: domain imports app
		violationContent := `package domain
import "example.com/archtest/internal/app"
func Bad() { app.Service() }`
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "domain", "violation.go"), []byte(violationContent), 0644))

		root := &cobra.Command{Use: "recac"}
		root.AddCommand(archCmd)

		output, err := executeCommand(root, "arch", tmpDir)
		assert.Error(t, err)
		assert.Contains(t, output, "Found 1 architecture violations")
		assert.Contains(t, output, "example.com/archtest/internal/domain (domain) imports example.com/archtest/internal/app (app)")
	})

	t.Run("Generate Config", func(t *testing.T) {
		// Create a clean subdir
		subDir := filepath.Join(tmpDir, "subdir")
		require.NoError(t, os.Mkdir(subDir, 0755))

		root := &cobra.Command{Use: "recac"}
		root.AddCommand(archCmd)

		// executeCommand doesn't change CWD, but runArch uses path arg now.
		_, err := executeCommand(root, "arch", subDir, "--generate")
		assert.NoError(t, err)
		// Output check skipped because generateDefaultArchConfig writes to stdout directly

		assert.FileExists(t, filepath.Join(subDir, ".recac-arch.yaml"))
	})
}
