package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchGraphCmd(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-arch-graph-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	err = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/archgraphtest\n\ngo 1.20\n"), 0644)
	require.NoError(t, err)

	// Create layers:
	// Domain: independent
	// App: imports Domain
	// Infra: imports Domain (Allowed)
	// Violation: Domain imports Infra (Forbidden)

	// internal/domain/model.go
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "domain"), 0755))
	domainContent := `package domain
import "example.com/archgraphtest/internal/infra" // Violation!
func Model() { infra.Db() }`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "domain", "model.go"), []byte(domainContent), 0644))

	// internal/infra/db.go
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "infra"), 0755))
	infraContent := `package infra
func Db() {}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "infra", "db.go"), []byte(infraContent), 0644))

	// internal/app/service.go
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "app"), 0755))
	appContent := `package app
import "example.com/archgraphtest/internal/domain"
func Service() { domain.Model() }`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "app", "service.go"), []byte(appContent), 0644))

	// Create config file
	// Regex must match the full package path: example.com/archgraphtest/internal/domain
	configContent := `layers:
  domain: ".*internal/domain.*"
  app: ".*internal/app.*"
  infra: ".*internal/infra.*"

rules:
  - from: "domain"
    allow: []
  - from: "app"
    allow: ["domain"]
  - from: "infra"
    allow: ["domain"]
`
	configFile := filepath.Join(tmpDir, ".recac-arch.yaml")
	require.NoError(t, os.WriteFile(configFile, []byte(configContent), 0644))

	// Setup Command
	root := &cobra.Command{Use: "recac"}
    // archCmd is a package-level variable in main, but we need to ensure it has graph added.
    // The init() function runs automatically when running tests in the package.
    // So archCmd should already have archGraphCmd.
    root.AddCommand(archCmd)

	output, err := executeCommand(root, "arch", "graph", tmpDir)
	assert.NoError(t, err)

	// Verify Output
	// 1. Mermaid Header
	assert.Contains(t, output, "graph TD")

	// 2. Nodes
	assert.Contains(t, output, "domain[\"domain\"]")
	assert.Contains(t, output, "app[\"app\"]")
	assert.Contains(t, output, "infra[\"infra\"]")

	// 3. Edges
	// App -> Domain (Allowed)
	assert.Contains(t, output, "app --> domain")

    // Domain -> Infra (Violation)
    assert.Contains(t, output, "domain --> infra")

    // 4. Styles
    // We expect app->domain (green) and domain->infra (red)
    // Sorted: app->domain comes first.

    // Check for Green
    assert.Contains(t, output, "linkStyle 0 stroke:#00ff00")

    // Check for Red
    assert.Contains(t, output, "linkStyle 1 stroke:#ff0000")
}
