package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// ApiExploreMockAgent implements agent.Agent
type ApiExploreMockAgent struct {
	mock.Mock
}

func (m *ApiExploreMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *ApiExploreMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestScanEndpointsWithAI(t *testing.T) {
	// Setup Mock
	mockAgent := new(ApiExploreMockAgent)
	jsonResponse := `[
		{
			"method": "GET",
			"path": "/api/users",
			"description": "List users",
			"headers": {"Accept": "application/json"}
		}
	]`
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(jsonResponse, nil)

	// Override factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// Create temp dir for scanning
	tempDir := t.TempDir()
	err := os.WriteFile(filepath.Join(tempDir, "main.go"), []byte("package main"), 0644)
	assert.NoError(t, err)

	// Test
	ctx := context.Background()
	endpoints, err := scanEndpointsWithAI(ctx, tempDir)

	assert.NoError(t, err)
	assert.Len(t, endpoints, 1)
	assert.Equal(t, "GET", endpoints[0].Method)
	assert.Equal(t, "/api/users", endpoints[0].Path)
}

func TestApiExplorerModel_Update(t *testing.T) {
	model := InitialApiExplorerModel()

	// Test Endpoint Scanned Msg
	eps := []DiscoveredEndpoint{
		{Method: "POST", Path: "/api/create", Description: "Create", Body: "{}"},
	}
	msg := endpointsScannedMsg{Endpoints: eps}

	updatedModel, _ := model.Update(msg)
	m := updatedModel.(ApiExplorerModel)

	assert.False(t, m.loading)
	assert.Len(t, m.endpoints, 1)
	assert.Equal(t, "/api/create", m.urlInput.Value()) // Should be selected automatically
	assert.Equal(t, "POST", m.currentMethod)
}
