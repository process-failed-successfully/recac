package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"recac/internal/agent"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

type MockTourAgent struct {
	Response string
}

func (m *MockTourAgent) Send(ctx context.Context, prompt string) (string, error) {
	return m.Response, nil
}

func (m *MockTourAgent) SendStream(ctx context.Context, prompt string, onChunk func(string)) (string, error) {
	return m.Response, nil
}

func TestGenerateTourStops(t *testing.T) {
	// Setup temporary directory with a dummy file
	tmpDir := t.TempDir()
	// Use a unique name to avoid collision with repo root files if CWD is not changed
	uniqueName := "TOUR_TEST_README.md"
	dummyFile := filepath.Join(tmpDir, uniqueName)
	if err := os.WriteFile(dummyFile, []byte("# Test Project"), 0644); err != nil {
		t.Fatal(err)
	}

	// Mock Agent Response
	stops := []TourStop{
		{
			TitleText:       "Welcome",
			DescriptionText: "Introduction",
			File:            uniqueName,
		},
	}
	jsonBytes, _ := json.Marshal(stops)
	mockAgent := &MockTourAgent{Response: string(jsonBytes)}

	// Mock Factory
	origFactory := agentClientFactory
	defer func() { agentClientFactory = origFactory }()

	agentClientFactory = func(ctx context.Context, provider, model, projectPath, projectName string) (agent.Agent, error) {
		return mockAgent, nil
	}

	// Test
	ctx := context.Background()
	result, err := generateTourStops(ctx, tmpDir, tmpDir, 1)

	assert.NoError(t, err)
	assert.Len(t, result, 1)
	assert.Equal(t, "Welcome", result[0].TitleText)
	// result[0].File should be absolute path because validate paths converts it
	assert.Equal(t, dummyFile, result[0].File)
}

func TestTourModel_Update(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	dummyFile := filepath.Join(tmpDir, "README.md")
	if err := os.WriteFile(dummyFile, []byte("# Test Content"), 0644); err != nil {
		t.Fatal(err)
	}

	stops := []TourStop{
		{
			TitleText:       "Stop 1",
			DescriptionText: "Desc 1",
			File:            dummyFile,
		},
	}

	items := make([]list.Item, len(stops))
	for i, s := range stops {
		items[i] = s
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	m := TourModel{
		list:     l,
		viewport: viewport.New(0, 0),
		stops:    stops,
	}

	// Initialize size so viewport can render
	m.list.SetSize(20, 10)
	m.viewport.Width = 40
	m.viewport.Height = 20

	// Or better, send WindowSizeMsg to trigger resize logic in Update
	sizeMsg := tea.WindowSizeMsg{Width: 80, Height: 24}
	newM, _ := m.Update(sizeMsg)
	m = newM.(TourModel)

	// Initial State: active = false (list focus)
	assert.False(t, m.active)

	// Simulate Enter key -> should toggle active
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newM, _ = m.Update(msg)
	tm := newM.(TourModel)
	assert.True(t, tm.active)
	// Viewport should have content
	// View() output might contain ANSI codes, but the content should be there.
	// Wait, viewport content is set via SetContent which is internal.
	// But View() renders it.
	assert.Contains(t, tm.viewport.View(), "Test Content")

	// Simulate Enter key again -> should toggle back
	newM, _ = tm.Update(msg)
	tm = newM.(TourModel)
	assert.False(t, tm.active)
}
