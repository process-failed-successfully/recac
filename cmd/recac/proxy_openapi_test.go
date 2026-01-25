package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"recac/internal/agent"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProxyOpenAPIGeneration(t *testing.T) {
	// Create a temporary directory for test files
	tmpDir := t.TempDir()
	recordingFile := filepath.Join(tmpDir, "recording.json")
	outputFile := filepath.Join(tmpDir, "openapi.yaml")

	// Create dummy interactions
	interactions := []Interaction{
		{
			Timestamp: time.Now(),
			Request: ReqDump{
				Method: "GET",
				URL:    "http://api.example.com/users/1",
				Headers: map[string][]string{
					"Accept": {"application/json"},
				},
			},
			Response: ResDump{
				Status: 200,
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
				Body: `{"id": 1, "name": "John Doe"}`,
			},
		},
		{
			Timestamp: time.Now(),
			Request: ReqDump{
				Method: "POST",
				URL:    "http://api.example.com/users",
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
				Body: `{"name": "Jane Doe"}`,
			},
			Response: ResDump{
				Status: 201,
				Headers: map[string][]string{
					"Content-Type": {"application/json"},
				},
				Body: `{"id": 2, "name": "Jane Doe"}`,
			},
		},
	}

	// Write interactions to file (JSONL format as used in proxy.go)
	f, err := os.Create(recordingFile)
	require.NoError(t, err)
	for _, i := range interactions {
		data, _ := json.Marshal(i)
		f.Write(data)
		f.WriteString("\n")
	}
	f.Close()

	// Mock the agent
	mockAgent := agent.NewMockAgent()
	expectedYAML := `openapi: 3.0.0
info:
  title: Generated API
  version: 1.0.0
paths:
  /users:
    post:
      responses:
        '201':
          description: Created
  /users/{id}:
    get:
      responses:
        '200':
          description: OK`
	mockAgent.SetResponse(expectedYAML)

	// Override the factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Execute the command using the helper from test_helpers_test.go
	output, err := executeCommand(rootCmd, "proxy",
		"--record", recordingFile,
		"--generate",
		"--format", "openapi",
		"--output", outputFile,
	)

	// Check for errors (executeCommand might capture error in err, or output)
	require.NoError(t, err, "Command execution failed: %s", output)

	// Verify output file
	require.FileExists(t, outputFile)
	content, err := os.ReadFile(outputFile)
	require.NoError(t, err)
	assert.Equal(t, expectedYAML, string(content))

	// Verify output stdout
	assert.Contains(t, output, "Generating OpenAPI Spec")
	assert.Contains(t, output, "Generated OpenAPI spec saved to")
}
