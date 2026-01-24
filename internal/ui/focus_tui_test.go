package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestFocusModel_Init(t *testing.T) {
	m := NewFocusModel(10*time.Minute, "Test Task")
	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestFocusModel_Update_Tick(t *testing.T) {
	duration := 10 * time.Second
	m := NewFocusModel(duration, "Test Task")

	// Simulate one tick
	msg := TickMsg(time.Now())
	newModel, cmd := m.Update(msg)

	m2 := newModel.(FocusModel)

	// Remaining should decrease by 1 second (logic in Update: m.Remaining -= time.Second)
	assert.Equal(t, duration-time.Second, m2.Remaining)
	assert.NotNil(t, cmd) // Should return next tick cmd
}

func TestFocusModel_Update_Pause(t *testing.T) {
	m := NewFocusModel(10*time.Minute, "Test Task")
	assert.False(t, m.Paused)

	// Send space key
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m2 := newModel.(FocusModel)
	assert.True(t, m2.Paused)

	// Send space key again
	newModel, _ = m2.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m3 := newModel.(FocusModel)
	assert.False(t, m3.Paused)
}

func TestFocusModel_Update_Finished(t *testing.T) {
	// 1 second duration
	m := NewFocusModel(1*time.Second, "Test Task")

	// Tick 1: Remaining becomes 0
	msg := TickMsg(time.Now())
	newModel, cmd := m.Update(msg)
	m2 := newModel.(FocusModel)

	assert.Equal(t, time.Duration(0), m2.Remaining)
	assert.Equal(t, StateFinished, m2.State)

	// Verify cmd produces QuitMsg
	assert.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}

func TestFocusModel_Update_Quit(t *testing.T) {
	m := NewFocusModel(10*time.Minute, "Test Task")

	// Send 'q'
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	m2 := newModel.(FocusModel)

	assert.True(t, m2.Quitting)
	// Verify cmd produces QuitMsg
	assert.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}

func TestFocusModel_View(t *testing.T) {
	m := NewFocusModel(10*time.Minute, "Test Task")
	// Set width/height for rendering
	m.width = 80
	m.height = 24

	view := m.View()
	assert.Contains(t, view, "RECAC FOCUS")
	assert.Contains(t, view, "Test Task")
	assert.Contains(t, view, "10:00")
}
