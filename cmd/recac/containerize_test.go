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

// MockAgentForContainerize is a mock agent
type MockAgentForContainerize struct {
	Response string
}

func (m *MockAgentForContainerize) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgentForContainerize) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestContainerize(t *testing.T) {
	// Save original factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	t.Run("Generate Files Successfully", func(t *testing.T) {
		tmpDir := t.TempDir()
		outputDir := filepath.Join(tmpDir, "out")

		// Prepare mock response
		mockResponse := `
Here are the files:

<file path="Dockerfile">
FROM alpine
CMD ["echo", "hello"]
</file>

<file path=".dockerignore">
node_modules
.git
</file>

<file path="docker-compose.yml">
version: "3"
services:
  app:
    build: .
</file>
`
		// Mock the factory
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgentForContainerize{Response: mockResponse}, nil
		}

		// Set global flag variables manually since we are calling runContainerize directly
		contOutputDir = outputDir
		contCompose = true
		contPort = "8080"
		contDB = "postgres"

		// Create dummy file to simulate project context
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "package.json"), []byte(`{"name":"test"}`), 0644))

		// Change CWD to tmpDir so the command scans the temp dir
		oldCwd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(oldCwd)

		// Execute
		err = runContainerize(containerizeCmd, []string{})
		require.NoError(t, err)

		// Verify Dockerfile
		dockerfile := filepath.Join(outputDir, "Dockerfile")
		assert.FileExists(t, dockerfile)
		content, err := os.ReadFile(dockerfile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "FROM alpine")

		// Verify .dockerignore
		ignore := filepath.Join(outputDir, ".dockerignore")
		assert.FileExists(t, ignore)
		content, err = os.ReadFile(ignore)
		require.NoError(t, err)
		assert.Contains(t, string(content), "node_modules")

		// Verify docker-compose.yml
		compose := filepath.Join(outputDir, "docker-compose.yml")
		assert.FileExists(t, compose)
		content, err = os.ReadFile(compose)
		require.NoError(t, err)
		assert.Contains(t, string(content), "version: \"3\"")
	})

	t.Run("Handle Malformed Response", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Mock factory to return invalid XML
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgentForContainerize{Response: "No XML tags here"}, nil
		}

		contOutputDir = tmpDir

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		err := runContainerize(containerizeCmd, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse agent response")
	})

	t.Run("Respect Force Flag", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Create existing file
		existingFile := filepath.Join(tmpDir, "Dockerfile")
		require.NoError(t, os.WriteFile(existingFile, []byte("OLD CONTENT"), 0644))

		mockResponse := `<file path="Dockerfile">NEW CONTENT</file>`

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgentForContainerize{Response: mockResponse}, nil
		}

		contOutputDir = tmpDir
		contForce = false // Should NOT overwrite

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		// 1. Run without force
		err := runContainerize(containerizeCmd, []string{})
		require.NoError(t, err)

		content, _ := os.ReadFile(existingFile)
		assert.Equal(t, "OLD CONTENT", string(content))

		// 2. Run with force
		contForce = true
		err = runContainerize(containerizeCmd, []string{})
		require.NoError(t, err)

		content, _ = os.ReadFile(existingFile)
		assert.Equal(t, "NEW CONTENT", string(content))
	})
}
