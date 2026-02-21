package main

import (
	"context"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"recac/internal/agent"
)

type TourMockAgent struct {
	Response string
}

func (m *TourMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *TourMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	onChunk(m.Response)
	return m.Response, nil
}

func TestTourModel_Navigation(t *testing.T) {
	// Setup Itinerary
	itinerary := &TourItinerary{
		Title:       "Test Tour",
		Description: "A test tour",
		Steps: []TourStep{
			{Title: "Step 1", File: "file1.go", Line: 10, Description: "Desc 1"},
			{Title: "Step 2", File: "file2.go", Line: 20, Description: "Desc 2"},
		},
	}

	m := initialTourModel()
	m.itinerary = itinerary
	m.loading = false
	m.ready = true
	m.width = 100
	m.height = 50

	// Initial State
	assert.Equal(t, 0, m.currentStep)

	// Update: Next Step (right arrow)
	msg := tea.KeyMsg{Type: tea.KeyRight}
	newM, _ := m.Update(msg)
	tourM := newM.(tourModel)
	assert.Equal(t, 1, tourM.currentStep)

	// Update: Next Step (at end)
	newM, _ = tourM.Update(msg)
	tourM = newM.(tourModel)
	assert.Equal(t, 1, tourM.currentStep) // Should stay at last step

	// Update: Prev Step (left arrow)
	msg = tea.KeyMsg{Type: tea.KeyLeft}
	newM, _ = tourM.Update(msg)
	tourM = newM.(tourModel)
	assert.Equal(t, 0, tourM.currentStep)
}

func TestGenerateItinerary(t *testing.T) {
	// Mock Agent
	mockResponse := `{
		"title": "Mock Tour",
		"description": "Mock Description",
		"steps": [
			{"title": "Mock Step", "file": "main.go", "line": 1, "description": "Mock Desc"}
		]
	}`

	// Override factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return &TourMockAgent{Response: mockResponse}, nil
	}

	// Run
	itinerary, err := generateItinerary()
	assert.NoError(t, err)
	assert.NotNil(t, itinerary)
	assert.Equal(t, "Mock Tour", itinerary.Title)
	assert.Equal(t, 1, len(itinerary.Steps))
	assert.Equal(t, "main.go", itinerary.Steps[0].File)
}
