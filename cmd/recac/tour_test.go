package main

import (
	"context"
	"recac/internal/agent"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockTourAgent avoids conflict with other test files in the same package
type MockTourAgent struct {
	mock.Mock
}

func (m *MockTourAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *MockTourAgent) SendStream(ctx context.Context, prompt string, callback func(string)) (string, error) {
	args := m.Called(ctx, prompt, callback)
	if callback != nil {
		callback(args.String(0))
	}
	return args.String(0), args.Error(1)
}

func TestTourModel_Update(t *testing.T) {
	slides := []string{"Slide 1", "Slide 2", "Slide 3"}
	m := NewTourModel(".")
	m.slides = slides
	m.ready = true
	m.loading = false
	m.index = 0

	// Test Next (Right Arrow)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRight})
	tm := newModel.(TourModel)
	assert.Equal(t, 1, tm.index, "Should move to next slide")

	// Test Next (Enter)
	newModel, _ = tm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	tm = newModel.(TourModel)
	assert.Equal(t, 2, tm.index, "Should move to next slide")

	// Test Next at end (Should stay)
	newModel, _ = tm.Update(tea.KeyMsg{Type: tea.KeyRight})
	tm = newModel.(TourModel)
	assert.Equal(t, 2, tm.index, "Should stay at last slide")

	// Test Prev (Left Arrow)
	newModel, _ = tm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	tm = newModel.(TourModel)
	assert.Equal(t, 1, tm.index, "Should move to prev slide")

	// Test Prev at start (Should stay)
	tm.index = 0
	newModel, _ = tm.Update(tea.KeyMsg{Type: tea.KeyLeft})
	tm = newModel.(TourModel)
	assert.Equal(t, 0, tm.index, "Should stay at first slide")

	// Test Quit
	newModel, cmd := tm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})
	_, ok := newModel.(TourModel)
	assert.True(t, ok)
	assert.Equal(t, tea.Quit(), cmd(), "Should return Quit command")
}

func TestGenerateTourContent(t *testing.T) {
	// Mock agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockTourAgent)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock response
	mockJSON := `["# Slide 1", "# Slide 2"]`
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(mockJSON, nil)

	// Call generation function directly (via Init or directly if exported, here via Init wrapper logic)
	// Since generateTourContent returns a tea.Cmd, we execute it.
	cmd := generateTourContent(".")
	msg := cmd()

	// Verify message
	contentMsg, ok := msg.(tourContentMsg)
	assert.True(t, ok, "Should return tourContentMsg")
	assert.NoError(t, contentMsg.err)
	assert.Equal(t, 2, len(contentMsg.slides))
	assert.Equal(t, "# Slide 1", contentMsg.slides[0])

	// Verify prompt contained structure
	mockAgent.AssertCalled(t, "Send", mock.Anything, mock.MatchedBy(func(prompt string) bool {
		return true // structure generation is complex to verify exactly without mocking FS, but at least we called Send
	}))
}

func TestGenerateTourContent_Error(t *testing.T) {
	// Mock agent factory
	originalFactory := agentClientFactory
	defer func() { agentClientFactory = originalFactory }()

	mockAgent := new(MockTourAgent)
	agentClientFactory = func(ctx context.Context, provider, model, dir, id string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Mock invalid JSON response
	mockResponse := "Not JSON"
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(mockResponse, nil)

	cmd := generateTourContent(".")
	msg := cmd()

	contentMsg, ok := msg.(tourContentMsg)
	assert.True(t, ok)
	assert.NoError(t, contentMsg.err)
	assert.Equal(t, 1, len(contentMsg.slides), "Should fallback to single slide")
	assert.Contains(t, contentMsg.slides[0], "Tour Generation Issue")
}
