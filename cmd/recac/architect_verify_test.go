package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchitectVerifyCmd(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-arch-verify-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create go.mod
	err = os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module example.com/archverify\n\ngo 1.20\n"), 0644)
	require.NoError(t, err)

	// Create architecture.yaml
	archContent := `
version: "1.0"
system_name: "TestSystem"
components:
  - id: "auth"
    type: "service"
    description: "Auth Service"
    consumes:
      - source: "db"
  - id: "db"
    type: "database"
    description: "Database"
  - id: "api"
    type: "service"
    description: "API Gateway"
    consumes:
      - source: "auth"
`
	archFile := filepath.Join(tmpDir, "architecture.yaml")
	require.NoError(t, os.WriteFile(archFile, []byte(archContent), 0644))

	// Create component directories
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "auth"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "db"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(tmpDir, "internal", "api"), 0755))

	// 1. Valid Code
	// API imports Auth (Allowed)
	apiContent := `package api
import "example.com/archverify/internal/auth"
func Handle() { auth.Login() }`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "api", "handler.go"), []byte(apiContent), 0644))

	authContent := `package auth
func Login() {}`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "auth", "login.go"), []byte(authContent), 0644))

	// Run Verify
	t.Run("Valid Architecture", func(t *testing.T) {
		// Mock command
		cmd := &cobra.Command{Use: "verify", RunE: runArchitectVerify}

		// We need to change CWD for analysis to work correctly or pass root via flags?
		// runArchitectVerify uses os.Getwd() for analysis root.
		// And args[0] for architecture file path.

		// We can't easily mock os.Getwd() in parallel tests.
		// But here we are sequential.
		// However, analysis.AnalyzeDependencies takes `DependencyOptions{Root: cwd}`.
		// runArchitectVerify calls os.Getwd().
		// We should probably allow passing root via flag to verify command?
		// Or chdir.

		cwd, _ := os.Getwd()
		defer os.Chdir(cwd)
		require.NoError(t, os.Chdir(tmpDir))

		// Execute
		// args: path to yaml
		err := runArchitectVerify(cmd, []string{"architecture.yaml"})
		assert.NoError(t, err)
	})

	// 2. Invalid Code
	// DB imports API (Forbidden)
	dbContent := `package db
import "example.com/archverify/internal/api"
func Query() { api.Handle() }`
	require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "internal", "db", "query.go"), []byte(dbContent), 0644))

	t.Run("Invalid Architecture", func(t *testing.T) {
		cwd, _ := os.Getwd()
		defer os.Chdir(cwd)
		require.NoError(t, os.Chdir(tmpDir))

		cmd := &cobra.Command{Use: "verify", RunE: runArchitectVerify}
		err := runArchitectVerify(cmd, []string{"architecture.yaml"})
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "architecture verification failed")
	})
}
