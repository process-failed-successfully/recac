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

// MockAgentForHelm is a mock agent
type MockAgentForHelm struct {
	Response string
}

func (m *MockAgentForHelm) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockAgentForHelm) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestHelm(t *testing.T) {
	// Save original factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	t.Run("Generate Chart Successfully", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Setup expectations
		chartName := "my-app"
		outputDir := filepath.Join(tmpDir, "charts", chartName)

		// Prepare mock response
		mockResponse := `
Here is the helm chart:

<file path="Chart.yaml">
apiVersion: v2
name: my-app
version: 0.1.0
</file>

<file path="values.yaml">
replicaCount: 1
image:
  repository: my-app
</file>

<file path="templates/deployment.yaml">
apiVersion: apps/v1
kind: Deployment
</file>
`
		// Mock the factory
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgentForHelm{Response: mockResponse}, nil
		}

		// Set global flag variables manually
		helmChartName = chartName
		helmOutputDir = outputDir
		helmPort = "8080"
		helmForce = false

		// Create dummy file to simulate project context
		require.NoError(t, os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte(`FROM alpine`), 0644))

		// Change CWD to tmpDir
		oldCwd, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(oldCwd)

		// Execute
		err = runHelm(helmCmd, []string{})
		require.NoError(t, err)

		// Verify Chart.yaml
		chartFile := filepath.Join(outputDir, "Chart.yaml")
		assert.FileExists(t, chartFile)
		content, err := os.ReadFile(chartFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "name: my-app")

		// Verify templates/deployment.yaml
		deployFile := filepath.Join(outputDir, "templates", "deployment.yaml")
		assert.FileExists(t, deployFile)
		content, err = os.ReadFile(deployFile)
		require.NoError(t, err)
		assert.Contains(t, string(content), "kind: Deployment")
	})

	t.Run("Handle Malformed Response", func(t *testing.T) {
		tmpDir := t.TempDir()

		// Mock factory to return invalid XML
		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgentForHelm{Response: "No XML tags here"}, nil
		}

		helmChartName = "bad-chart"
		helmOutputDir = filepath.Join(tmpDir, "bad-chart")

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		err := runHelm(helmCmd, []string{})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "failed to parse agent response")
	})

	t.Run("Respect Force Flag", func(t *testing.T) {
		tmpDir := t.TempDir()
		chartName := "force-test"
		outputDir := filepath.Join(tmpDir, chartName)

		// Create existing file
		require.NoError(t, os.MkdirAll(outputDir, 0755))
		existingFile := filepath.Join(outputDir, "values.yaml")
		require.NoError(t, os.WriteFile(existingFile, []byte("OLD VALUES"), 0644))

		mockResponse := `<file path="values.yaml">NEW VALUES</file>`

		agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
			return &MockAgentForHelm{Response: mockResponse}, nil
		}

		helmChartName = chartName
		helmOutputDir = outputDir
		helmForce = false // Should NOT overwrite

		oldCwd, _ := os.Getwd()
		os.Chdir(tmpDir)
		defer os.Chdir(oldCwd)

		// 1. Run without force
		err := runHelm(helmCmd, []string{})
		require.NoError(t, err)

		content, _ := os.ReadFile(existingFile)
		assert.Equal(t, "OLD VALUES", string(content))

		// 2. Run with force
		helmForce = true
		err = runHelm(helmCmd, []string{})
		require.NoError(t, err)

		content, _ = os.ReadFile(existingFile)
		assert.Equal(t, "NEW VALUES", string(content))
	})
}
