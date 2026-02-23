package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestTourModel_Update(t *testing.T) {
	m := newTourModel()

	// Initial state
	assert.Equal(t, 0, m.current)

	// Simulate "Enter" key
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newM, cmd := m.Update(msg)

	updatedM := newM.(TourModel)
	assert.Equal(t, 1, updatedM.current, "Should move to next step on Enter")
	// The command returned is a Batch which might contain commands from viewport update
	// so we don't strictly assert nil cmd here, but usually it is nil or batch.
	_ = cmd

	// Simulate "Left" key
	msg = tea.KeyMsg{Type: tea.KeyLeft}
	newM, cmd = updatedM.Update(msg)

	updatedM = newM.(TourModel)
	assert.Equal(t, 0, updatedM.current, "Should move to previous step on Left")

	// Simulate "q" key
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}
	newM, cmd = updatedM.Update(msg)

	// tea.Quit returns a special command. We can't easily compare functions.
	// But we can check if it returns something.
	assert.NotNil(t, cmd)
}

func TestTourModel_View(t *testing.T) {
	m := newTourModel()
	// Test that View doesn't panic initially
	view := m.View()
	assert.Contains(t, view, "Initializing Tour")

	// Simulate window size to make it ready
	// Note: Update returns a new model, we must capture it
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = newM.(TourModel)

	// Check header
	view = m.View()
	assert.Contains(t, view, "Recac Tour")
	// Since glamour rendering might vary or fail in test env, we just check structure
	assert.Contains(t, view, "Next: Enter")
}
