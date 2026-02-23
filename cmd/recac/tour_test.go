package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"recac/internal/agent"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type TestMockAgent struct {
	mock.Mock
}

func (m *TestMockAgent) Send(ctx context.Context, prompt string) (string, error) {
	args := m.Called(ctx, prompt)
	return args.String(0), args.Error(1)
}

func (m *TestMockAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	args := m.Called(ctx, prompt, onChunk)
	return args.String(0), args.Error(1)
}

func TestGenerateTour(t *testing.T) {
	// 1. Setup Temp Dir
	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main\nfunc main() {}"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# My Project"), 0644)

	// 2. Setup Mock Agent
	mockAgent := new(TestMockAgent)
	mockResponse := `[
		{
			"file": "main.go",
			"title": "Main Entry Point",
			"description": "This is where the app starts."
		},
		{
			"file": "README.md",
			"title": "Documentation",
			"description": "Project overview."
		}
	]`

	// The prompt contains file tree content, so exact match is hard. Use mock.Anything.
	mockAgent.On("Send", mock.Anything, mock.Anything).Return(mockResponse, nil)

	// 3. Override Factory
	origFactory := agentClientFactory
	agentClientFactory = func(ctx context.Context, provider, model, path, project string) (agent.Agent, error) {
		return mockAgent, nil
	}
	defer func() { agentClientFactory = origFactory }()

	// 4. Run
	stops, err := generateTour(context.Background(), tmpDir)

	// 5. Assert
	assert.NoError(t, err)
	assert.Len(t, stops, 2)
	assert.Equal(t, "main.go", stops[0].File)
	assert.Equal(t, "Main Entry Point", stops[0].TitleText)

	// Verify agent was called
	mockAgent.AssertExpectations(t)
}

func TestTourModel_Selection(t *testing.T) {
	// Setup Model
	stops := []TourStop{
		{File: "A", TitleText: "Title A", Desc: "Desc A"},
		{File: "B", TitleText: "Title B", Desc: "Desc B"},
	}

	items := make([]list.Item, len(stops))
	for i, stop := range stops {
		items[i] = stop
	}

	l := list.New(items, list.NewDefaultDelegate(), 20, 10)

	// Create renderer for test
	// Note: We might need to mock this if we want to avoid actual rendering,
	// but using default renderer is fine for unit test if we don't assert output content strictly.
	// Actually, bubbletea/glamour might panic if TERM is not set or valid.
	// But let's try initializing it.
	renderer, _ := glamour.NewTermRenderer()

	m := TourModel{
		stops:    stops,
		list:     l,
		ready:    true,
		width:    100,
		height:   40,
		renderer: renderer,
	}

	// Initial State: Selection is index 0 ("Title A")
	assert.Equal(t, "Title A", m.list.SelectedItem().(TourStop).TitleText)

	// Simulate "Down" key press
	msg := tea.KeyMsg{Type: tea.KeyDown}

	// Update
	// Note: Update returns tea.Model, we cast back to TourModel (value)
	newModel, _ := m.Update(msg)
	tm := newModel.(TourModel)

	// Verify selection changed to "Title B"
	assert.Equal(t, "Title B", tm.list.SelectedItem().(TourStop).TitleText)

	// Verify selectedTitle state tracked
	assert.Equal(t, "Title B", tm.selectedTitle)
}
