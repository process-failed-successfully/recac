package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestInitialTourModel(t *testing.T) {
	m := initialTourModel()
	assert.NotEmpty(t, m.slides)
	assert.Equal(t, 0, m.current)
	// Renderer might be nil depending on env, but we check if it handles it in View
}

func TestTourModel_Update(t *testing.T) {
	m := initialTourModel()

	// Test Next Slide
	nextMsg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}} // Space
	newModel, _ := m.Update(nextMsg)
	tm := newModel.(tourModel)
	assert.Equal(t, 1, tm.current)

	// Test Previous Slide
	prevMsg := tea.KeyMsg{Type: tea.KeyLeft}
	newModel, _ = tm.Update(prevMsg)
	tm = newModel.(tourModel)
	assert.Equal(t, 0, tm.current)

	// Test Quit
	quitMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newModel, cmd := m.Update(quitMsg)
	tm = newModel.(tourModel)
	assert.True(t, tm.quitting)
	assert.Equal(t, tea.Quit(), cmd())
}

func TestTourModel_View(t *testing.T) {
	m := initialTourModel()

	// Ensure View doesn't panic
	output := m.View()
	assert.NotEmpty(t, output)

	// If renderer is working, it should contain some formatted text or at least the content
	// We check for a known string from the first slide
	assert.Contains(t, output, "Welcome to RECAC")

	// Test Quit View
	m.quitting = true
	output = m.View()
	assert.Contains(t, output, "Bye")
}

func TestTourModel_WindowSize(t *testing.T) {
	m := initialTourModel()

	// Simulate resize
	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newModel, _ := m.Update(resizeMsg)
	tm := newModel.(tourModel)

	assert.Equal(t, 100, tm.width)
	assert.Equal(t, 50, tm.height)
}
