package main

import (
	"context"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockAgent implementation for testing
type MockTourAgent struct {
	Response string
}

func (m *MockTourAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockTourAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestLoadSaveTour(t *testing.T) {
	// Setup temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "tour.json")

	// Save original global var
	origTourFile := tourFile
	tourFile = tmpFile
	defer func() { tourFile = origTourFile }()

	slides := []TourSlide{
		{
			Title:       "Test Slide",
			Filepath:    "test.go",
			Description: "This is a test description.",
		},
	}

	// Test Save
	err := saveTour(slides)
	require.NoError(t, err)

	// Test Load
	loadedSlides, err := loadTour()
	require.NoError(t, err)
	assert.Equal(t, slides, loadedSlides)
}

func TestGenerateTour(t *testing.T) {
	// Mock factory
	origFactory := tourAgentFactory
	defer func() { tourAgentFactory = origFactory }()

	mockResponse := `[
		{
			"title": "Generated Slide",
			"filepath": "gen.go",
			"description": "Generated description"
		}
	]`

	tourAgentFactory = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return &MockTourAgent{Response: mockResponse}, nil
	}

	// Test Generate
	slides, err := generateTour(context.Background())
	require.NoError(t, err)
	assert.Len(t, slides, 1)
	assert.Equal(t, "Generated Slide", slides[0].Title)
}

func TestTourModel_Update(t *testing.T) {
	slides := []TourSlide{
		{Title: "Slide 1", Description: "Desc 1"},
		{Title: "Slide 2", Description: "Desc 2"},
	}

	m := initialModel(slides)

	// Verify initial state
	assert.Equal(t, 0, m.current)

	// Simulate 'n' key press (next)
	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}}
	newM, _ := m.Update(msg)
	newModel := newM.(tourModel)
	assert.Equal(t, 1, newModel.current)

	// Simulate 'p' key press (prev)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'p'}}
	newM, _ = newModel.Update(msg)
	newModel = newM.(tourModel)
	assert.Equal(t, 0, newModel.current)

	// Simulate 'q' key press (quit)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := newModel.Update(msg)
	assert.Equal(t, tea.Quit(), cmd())
}

func TestTourModel_View(t *testing.T) {
	slides := []TourSlide{
		{Title: "Slide 1", Description: "Desc 1"},
	}
	m := initialModel(slides)

	// Not ready yet (no window size msg)
	assert.Contains(t, m.View(), "Initializing")

	// Send window size
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(tourModel)

	// Should render content
	view := m.View()
	assert.Contains(t, view, "Slide 1")
	assert.Contains(t, view, "Desc 1")
}

func TestTourModel_Init(t *testing.T) {
	slides := []TourSlide{
		{Title: "Slide 1", Description: "Desc 1"},
	}
	m := initialModel(slides)
	cmd := m.Init()
	assert.Nil(t, cmd) // Init should return nil
}

func TestRunTour_EmptySlides(t *testing.T) {
	// Provide empty json
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "tour.json")

	// Save original global var
	origTourFile := tourFile
	tourFile = tmpFile
	defer func() { tourFile = origTourFile }()

	// Write empty array to the file
	os.WriteFile(tmpFile, []byte("[]"), 0644)

	cmd := &cobra.Command{}
	err := runTour(cmd, []string{})

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "tour is empty")
}

func TestRunTour_Generate(t *testing.T) {
	// Setup temp file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "tour.json")

	// Save original global var
	origTourFile := tourFile
	tourFile = tmpFile
	defer func() { tourFile = origTourFile }()

	// Mock factory
	origFactory := tourAgentFactory
	defer func() { tourAgentFactory = origFactory }()

	mockResponse := `[
		{
			"title": "Generated Slide",
			"filepath": "gen.go",
			"description": "Generated description"
		}
	]`

	tourAgentFactory = func(provider, apiKey, model, workDir, project string) (agent.Agent, error) {
		return &MockTourAgent{Response: mockResponse}, nil
	}

	// We can't really mock os.Stdin properly for BubbleTea's termios setup
	// in some environments, which can cause the test to hang indefinitely.
	// Instead of calling runTour which calls tea.NewProgram(...).Run(),
	// we will run tea.NewProgram with a custom input that exits immediately.
	// But since runTour is hardcoded to use tea.NewProgram without options we can override,
	// let's just test that the slides are saved and generated properly.

	// Because runTour is hard to test directly due to tea.NewProgram blocking,
	// we will manually test the logic that runTour uses.

	slides, err := loadTour()
	assert.Error(t, err) // File doesn't exist yet

	slides, err = generateTour(context.Background())
	assert.NoError(t, err)
	assert.Len(t, slides, 1)

	err = saveTour(slides)
	assert.NoError(t, err)

	// Check if file was saved
	savedSlides, err := loadTour()
	assert.NoError(t, err)
	assert.Len(t, savedSlides, 1)
	assert.Equal(t, "Generated Slide", savedSlides[0].Title)

	// Since we verified generateTour, saveTour, and loadTour,
	// we've covered the core logic of runTour. We skip the TUI Run part.
}
