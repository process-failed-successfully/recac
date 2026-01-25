package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ApiMockAgent implementation
type ApiMockAgent struct {
	mock.Mock
}

func (m *ApiMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *ApiMockAgent) SendStream(ctx context.Context, prompt string, onToken func(string)) (string, error) {
	args := m.Called(ctx, prompt, onToken)
	return args.String(0), args.Error(1)
}

func TestApiCommand_FindEndpoints(t *testing.T) {
	// 1. Setup Test Data
	tempDir := t.TempDir()
	code := `package main

import "net/http"

func main() {
	http.HandleFunc("/users", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("users"))
	})

	http.Handle("/admin", http.HandlerFunc(adminHandler))
}

func adminHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("admin"))
}
`
	err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte(code), 0644)
	assert.NoError(t, err)

	// 2. Run findEndpoints
	endpoints, err := findEndpoints(tempDir)
	assert.NoError(t, err)

	// 3. Verify
	assert.Len(t, endpoints, 2)

	// Map by path for easy verification
	epMap := make(map[string]Endpoint)
	for _, ep := range endpoints {
		epMap[ep.Path] = ep
	}

	if ep, ok := epMap["/users"]; ok {
		assert.Equal(t, "ANY", ep.Method)
		assert.Contains(t, ep.HandlerCode, "w.Write([]byte(\"users\"))")
	} else {
		t.Error("Endpoint /users not found")
	}

	if ep, ok := epMap["/admin"]; ok {
		assert.Equal(t, "ANY", ep.Method)
		assert.Contains(t, ep.HandlerCode, "w.Write([]byte(\"admin\"))")
	} else {
		t.Error("Endpoint /admin not found")
	}
}

func TestApiCommand_GenerateSpec(t *testing.T) {
	// Mock Agent Factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(ApiMockAgent)
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Prepare Mock Response
	mockYaml := `
get:
  summary: Get Users
  responses:
    '200':
      description: OK
`
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(mockYaml, nil)

	// endpoints
	endpoints := []Endpoint{
		{Method: "GET", Path: "/users", HandlerCode: "func() {}"},
	}

	// Capture output
	// We check file creation
	outputFile := filepath.Join(t.TempDir(), "openapi.yaml")
	apiOutputDir = outputFile

	err := generateSpec(apiCmd, endpoints)
	assert.NoError(t, err)

	// Verify File Created
	content, err := os.ReadFile(outputFile)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "openapi: 3.0.0")
	assert.Contains(t, string(content), "summary: Get Users")

	// Clean up
	apiOutputDir = ""
}
