package main

import (
	"context"
	"encoding/json"
	"os"
	"recac/internal/agent"
	"testing"

	"github.com/stretchr/testify/assert"
)

// MockTourAgent implements agent.Agent interface
type MockTourAgent struct {
	Response string
	Err      error
}

func (m *MockTourAgent) Send(ctx context.Context, content string) (string, error) {
	return m.Response, m.Err
}

func (m *MockTourAgent) SendStream(ctx context.Context, content string, onChunk func(string)) (string, error) {
	return m.Response, m.Err
}

func TestTourCmd_GenerateItinerary(t *testing.T) {
	// Setup Mock Agent
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockResponse := `{
  "project_name": "Test Project",
  "overview": "A test project",
  "steps": [
    {
      "title": "Start Here",
      "file": "README.md",
      "description": "The main readme"
    }
  ]
}`

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &MockTourAgent{Response: mockResponse}, nil
	}

	// Use TempDir to avoid side effects
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	assert.NoError(t, err)
	err = os.Chdir(tmpDir)
	assert.NoError(t, err)
	defer os.Chdir(originalWd)

	// Create a dummy README.md for context generation
	err = os.WriteFile("README.md", []byte("# Test Project"), 0644)
	assert.NoError(t, err)

	// Test
	ctx := context.Background()
	itinerary, err := generateItinerary(ctx)

	assert.NoError(t, err)
	assert.Equal(t, "Test Project", itinerary.ProjectName)
	assert.Equal(t, 1, len(itinerary.Steps))
	assert.Equal(t, "Start Here", itinerary.Steps[0].TitleText)
	assert.Equal(t, "README.md", itinerary.Steps[0].File)
}

func TestTourCmd_LoadItinerary(t *testing.T) {
	// Use TempDir to avoid side effects
	tmpDir := t.TempDir()
	originalWd, err := os.Getwd()
	assert.NoError(t, err)
	err = os.Chdir(tmpDir)
	assert.NoError(t, err)
	defer os.Chdir(originalWd)

	// Setup .recac/tour.json
	err = os.MkdirAll(".recac", 0755)
	assert.NoError(t, err)

	tourContent := `{
  "project_name": "Loaded Project",
  "overview": "Loaded from file",
  "steps": [
    {
      "title": "Loaded Step",
      "file": "main.go",
      "description": "Loaded description"
    }
  ]
}`
	err = os.WriteFile(".recac/tour.json", []byte(tourContent), 0644)
	assert.NoError(t, err)

	// Verify JSON structure
	var itinerary TourItinerary
	err = json.Unmarshal([]byte(tourContent), &itinerary)
	assert.NoError(t, err)
	assert.Equal(t, "Loaded Project", itinerary.ProjectName)
	assert.Equal(t, "Loaded Step", itinerary.Steps[0].TitleText)
}
