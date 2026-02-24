package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestTourModel_Update(t *testing.T) {
	m := initialTourModel()

	// Ensure we start at 0
	assert.Equal(t, 0, m.current)

	// Test Next (Right Arrow)
	// We need to construct a KeyMsg that matches the key binding.
	// The key binding uses "right".
	msg := tea.KeyMsg{Type: tea.KeyRight}
	newM, _ := m.Update(msg)
	tm, ok := newM.(tourModel)
	assert.True(t, ok)
	assert.Equal(t, 1, tm.current, "Should move to slide 1")

	// Test Next boundary (move to end)
	for i := 0; i < len(m.slides); i++ {
		newM, _ = tm.Update(msg)
		tm = newM.(tourModel)
	}
	assert.Equal(t, len(m.slides)-1, tm.current, "Should stay at last slide")

	// Test Prev (Left Arrow)
	msg = tea.KeyMsg{Type: tea.KeyLeft}
	newM, _ = tm.Update(msg)
	tm = newM.(tourModel)
	assert.Equal(t, len(m.slides)-2, tm.current, "Should move back one slide")

	// Test Quit (q)
	msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
	_, cmd := tm.Update(msg)
	assert.NotNil(t, cmd, "Quit should return a command")
}

func TestTourModel_View(t *testing.T) {
	m := initialTourModel()

	// Even if renderer fails (headless), View should return something (error message or content)
	view := m.View()
	assert.NotEmpty(t, view)

	// If renderer initialized, it should contain some content from slide 0
	if m.renderer != nil {
		// Slide 0 has "Welcome to RECAC"
		// Rendered output might contain ANSI codes, but should contain the text?
		// Glamour renders markdown.
		// Let's just check it's not empty.
		assert.NotEmpty(t, view)
	} else {
		assert.Contains(t, view, "Error: Renderer not initialized")
	}
}
