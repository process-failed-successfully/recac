package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockAgent implementation for this test file
type MockAgentRadar struct {
	mock.Mock
}

func (m *MockAgentRadar) Send(ctx context.Context, content string) (string, error) {
	args := m.Called(ctx, content)
	return args.String(0), args.Error(1)
}

func (m *MockAgentRadar) SendStream(ctx context.Context, content string, callback func(string)) (string, error) {
	args := m.Called(ctx, content, callback)
	return args.String(0), args.Error(1)
}

func TestRadarCmd_Heuristic(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()

	// Create go.mod
	goMod := `module example.com/test
go 1.21
require (
	github.com/gin-gonic/gin v1.9.0
)
`
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte(goMod), 0644)

	// Create Dockerfile
	os.WriteFile(filepath.Join(tmpDir, "Dockerfile"), []byte("FROM alpine"), 0644)

	// Override factory to force failure (fallback to heuristic)
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, fmt.Errorf("no agent")
	}
	defer func() { agentClientFactory = oldFactory }()

	// Capture output
	out, err := executeCommand(rootCmd, "radar", tmpDir, "--json")
	assert.NoError(t, err)

	// Extract JSON from output (since helper merges stdout/stderr)
	jsonStart := strings.Index(out, "{")
	if jsonStart != -1 {
		out = out[jsonStart:]
	}

	var output RadarOutput
	err = json.Unmarshal([]byte(out), &output)
	assert.NoError(t, err)

	// Verify heuristic items
	foundGin := false
	foundDocker := false
	for _, item := range output.Items {
		if item.Name == "Gin" && item.Quadrant == "Languages & Frameworks" {
			foundGin = true
		}
		if item.Name == "Docker" && item.Quadrant == "Platforms" {
			foundDocker = true
		}
	}
	assert.True(t, foundGin, "Should detect Gin via heuristic")
	assert.True(t, foundDocker, "Should detect Docker via heuristic")
}

func TestRadarCmd_AI(t *testing.T) {
	// Setup temp dir
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module foo\nrequire github.com/gin-gonic/gin v1.0"), 0644)

	// Mock Agent
	mockAgent := new(MockAgentRadar)
	expectedJSON := `[
		{ "name": "Gin", "quadrant": "Languages & Frameworks", "ring": "Adopt", "description": "Web Framework" }
	]`
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(expectedJSON, nil)

	// Override factory
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = oldFactory }()

	// Execute
	out, err := executeCommand(rootCmd, "radar", tmpDir, "--json")
	assert.NoError(t, err)

	// Extract JSON
	jsonStart := strings.Index(out, "{")
	if jsonStart != -1 {
		out = out[jsonStart:]
	}

	var output RadarOutput
	err = json.Unmarshal([]byte(out), &output)
	assert.NoError(t, err)

	assert.Equal(t, 1, len(output.Items))
	assert.Equal(t, "Gin", output.Items[0].Name)
	assert.Equal(t, "Adopt", output.Items[0].Ring)
}

func TestRadarCmd_HTML(t *testing.T) {
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "go.mod"), []byte("module foo"), 0644)

	// Override factory (heuristic)
	oldFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return nil, fmt.Errorf("no agent")
	}
	defer func() { agentClientFactory = oldFactory }()

	// Execute
	_, err := executeCommand(rootCmd, "radar", tmpDir, "--html")
	assert.NoError(t, err)

	// Check file existence
	htmlPath := filepath.Join(tmpDir, "radar.html")
	content, err := os.ReadFile(htmlPath)
	assert.NoError(t, err)
	assert.Contains(t, string(content), "<html>")
	assert.Contains(t, string(content), "Tech Radar")
}

func TestRadarHTMLGeneration(t *testing.T) {
	output := RadarOutput{
		Date: "2024-01-01",
		Items: []RadarItem{
			{Name: "Go", Quadrant: "Languages & Frameworks", Ring: "Adopt", Description: "Core"},
			{Name: "Rust", Quadrant: "Languages & Frameworks", Ring: "Trial", Description: "Experiment"},
		},
	}

	tmpDir := t.TempDir()
	err := generateRadarHTML(tmpDir, output)
	assert.NoError(t, err)

	content, _ := os.ReadFile(filepath.Join(tmpDir, "radar.html"))
	s := string(content)

	assert.Contains(t, s, "Go")
	assert.Contains(t, s, "Rust")
	assert.Contains(t, s, "ring-adopt")
	assert.Contains(t, s, "ring-trial")
}
