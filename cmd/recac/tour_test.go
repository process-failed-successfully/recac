package main

import (
	"context"
	"testing"

	"recac/internal/agent"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTourAgentClient avoids conflict with other tests
type MockTourAgentClient struct {
	mock.Mock
}

func (m *MockTourAgentClient) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockTourAgentClient) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	return args.String(0), args.Error(1)
}

func TestGenerateTour(t *testing.T) {
	// Mock agent
	mockAgent := new(MockTourAgentClient)
	originalFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = originalFactory }()

	// Mock response
	jsonResponse := `[
		{"title": "Intro", "description": "Welcome", "file": "README.md"},
		{"title": "Core", "description": "The Core", "file": "main.go"}
	]`

	// We expect the prompt to contain certain keywords
	mockAgent.On("Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return true // Accept any prompt for now, or check for "CODEBASE CONTEXT"
	})).Return(jsonResponse, nil)

	ctx := context.Background()
	slides, err := generateTour(ctx, "")

	assert.NoError(t, err)
	assert.Len(t, slides, 2)
	assert.Equal(t, "Intro", slides[0].Title)

	mockAgent.AssertExpectations(t)
}

func TestTourModel_Update(t *testing.T) {
	slides := []TourSlide{
		{Title: "Slide 1", Description: "Desc 1"},
		{Title: "Slide 2", Description: "Desc 2"},
	}

	m := TourModel{
		Slides: slides,
		Index:  0,
		// Viewport and Renderer are nil/zero
	}

	// Test Next
	msg := tea.KeyMsg{Type: tea.KeyRight}
	newM, _ := m.Update(msg)
	tm1 := newM.(TourModel)

	assert.Equal(t, 1, tm1.Index)

	// Test Prev (using the updated model)
	msg = tea.KeyMsg{Type: tea.KeyLeft}
	newM2, _ := tm1.Update(msg)
	tm2 := newM2.(TourModel)
	assert.Equal(t, 0, tm2.Index)

	// Test Quit
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := m.Update(msg)

	// tea.Quit returns a Cmd that returns a tea.QuitMsg
	if cmd != nil {
		msg := cmd()
		_, ok := msg.(tea.QuitMsg)
		assert.True(t, ok, "Expected QuitMsg")
	} else {
		t.Error("Expected command")
	}
}
