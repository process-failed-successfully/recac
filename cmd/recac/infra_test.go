package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAgentForInfra is a mock agent
type MockAgentForInfra struct {
	Response string
}

func (m *MockAgentForInfra) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgentForInfra) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestInfraCmd(t *testing.T) {
	// Save original factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	t.Run("Generate Terraform Files", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputDir := filepath.Join(tmpDir, "infra")

		// Mock Response
		mockResponse := `
Here is the Terraform code:

<file path="main.tf">
provider "aws" {
  region = "us-west-2"
}
</file>

<file path="variables.tf">
variable "instance_type" {
  default = "t2.micro"
}
</file>
`
		// Mock Factory
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgentForInfra{Response: mockResponse}, nil
		}

		// Set flags
		infraOut = outputDir
		infraType = "terraform"
		infraProvider = "aws"
		infraDesc = "Test Infra"
		infraForce = false

		// Change CWD
		oldCwd, _ := os.Getwd()
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(oldCwd)

		// Run
		err := runInfra(infraCmd, []string{})
		require.NoError(t, err)

		// Verify main.tf
		mainTF := filepath.Join(outputDir, "main.tf")
		assert.FileExists(t, mainTF)
		content, _ := os.ReadFile(mainTF)
		assert.Contains(t, string(content), "provider \"aws\"")

		// Verify variables.tf
		varsTF := filepath.Join(outputDir, "variables.tf")
		assert.FileExists(t, varsTF)
		content, _ = os.ReadFile(varsTF)
		assert.Contains(t, string(content), "variable \"instance_type\"")
	})

	t.Run("Handle Parse Error", func(t *testing.T) {
		tmpDir := t.TempDir()

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgentForInfra{Response: "Invalid response format"}, nil
		}

		infraOut = tmpDir
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		err := runInfra(infraCmd, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse agent response")
	})

	t.Run("Respect Force Flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputDir := filepath.Join(tmpDir, "infra")
		require.NoError(t, os.MkdirAll(outputDir, 0755))

		// Create existing file
		existingFile := filepath.Join(outputDir, "main.tf")
		require.NoError(t, os.WriteFile(existingFile, []byte("OLD CONFIG"), 0644))

		mockResponse := `<file path="main.tf">NEW CONFIG</file>`

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgentForInfra{Response: mockResponse}, nil
		}

		infraOut = outputDir
		infraForce = false // Should NOT overwrite

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		// 1. Run without force
		err := runInfra(infraCmd, []string{})
		require.NoError(t, err)
		content, _ := os.ReadFile(existingFile)
		assert.Equal(t, "OLD CONFIG", string(content))

		// 2. Run with force
		infraForce = true
		err = runInfra(infraCmd, []string{})
		require.NoError(t, err)
		content, _ = os.ReadFile(existingFile)
		assert.Equal(t, "NEW CONFIG\n", string(content))
	})

	t.Run("Prevent Path Traversal", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputDir := filepath.Join(tmpDir, "infra")

		// Malicious response
		mockResponse := `<file path="../evil.txt">EVIL</file><file path="/etc/passwd">EVIL</file>`

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgentForInfra{Response: mockResponse}, nil
		}

		infraOut = outputDir
		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		err := runInfra(infraCmd, []string{})
		require.NoError(t, err)

		// Verify files were NOT written
		// ../evil.txt should not exist relative to outputDir
		assert.NoFileExists(t, filepath.Join(tmpDir, "evil.txt"))
		// /etc/passwd obviously shouldn't be touched, but we can check if it logged a warning (harder to test without capturing stderr)
		// But essentially, the run should succeed (not crash) and not write the file.
	})
}
