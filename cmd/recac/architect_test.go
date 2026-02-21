package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"recac/internal/cmdutils"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestArchitectCmd(t *testing.T) {
	// Setup temp dir
	tmpDir, err := os.MkdirTemp("", "recac-architect-test-")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a dummy spec file
	specFile := filepath.Join(tmpDir, "app_spec.txt")
	err = os.WriteFile(specFile, []byte("Test Spec"), 0644)
	require.NoError(t, err)

	outDir := filepath.Join(tmpDir, ".recac/architecture")

	// Mock Agent
	mockAgent := agent.NewMockAgent()
	mockResponse := `{
		"architecture.yaml": "system_name: TestSystem\nversion: \"1.0\"\ndescription: Test Description\ncomponents:\n  - id: api\n    type: service\n    description: API Service\n",
		"contracts.yaml": "contracts:\n  - consumer: web\n    provider: api\n"
	}`
	mockAgent.SetResponse(mockResponse)

	// Override GetAgentClient factory
	originalGetAgentClient := cmdutils.GetAgentClient
	cmdutils.GetAgentClient = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { cmdutils.GetAgentClient = originalGetAgentClient }()

	// Execute command
	root := &cobra.Command{Use: "recac"}
	root.AddCommand(architectCmd)

	// We need to reset flags because architectCmd is a global variable
	architectCmd.Flags().Set("spec", specFile)
	architectCmd.Flags().Set("out", outDir)

	output, err := executeCommand(root, "architect", "--spec", specFile, "--out", outDir)
	assert.NoError(t, err)
	assert.Contains(t, output, "Architecting system...")
	assert.Contains(t, output, "Wrote architecture.yaml")
	assert.Contains(t, output, "Validating architecture...")
	assert.Contains(t, output, "SUCCESS: Architecture is valid.")

	// Verify files created
	assert.FileExists(t, filepath.Join(outDir, "architecture.yaml"))
	assert.FileExists(t, filepath.Join(outDir, "contracts.yaml"))

	// Verify content
	content, err := os.ReadFile(filepath.Join(outDir, "architecture.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(content), "system_name: TestSystem")
}

func TestBasePathFS(t *testing.T) {
	tmpDir := t.TempDir()
	fs := &BasePathFS{Base: tmpDir}
	err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644)
	require.NoError(t, err)

	info, err := fs.Stat("test.txt")
	assert.NoError(t, err)
	assert.Equal(t, "test.txt", info.Name())
}
