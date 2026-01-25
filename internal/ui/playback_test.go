package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestLogEntry_Methods(t *testing.T) {
	tm := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	e := LogEntry{
		Time:    tm,
		Level:   "INFO",
		Msg:     "Test message",
		Content: "Pretty content",
	}

	assert.Equal(t, "[INFO] Test message", e.Title())
	assert.Equal(t, "12:00:00.000", e.Description())
	assert.Equal(t, "INFO Test message", e.FilterValue())
}

func TestParseLogLines(t *testing.T) {
	data := []byte(`
{"time":"2023-01-01T12:00:00Z","level":"INFO","msg":"JSON Log","key":"value"}
Plain Text Log
	`)

	entries, err := ParseLogLines(data)
	assert.NoError(t, err)
	assert.Len(t, entries, 2)

	// Check JSON entry
	assert.Equal(t, "INFO", entries[0].Level)
	assert.Equal(t, "JSON Log", entries[0].Msg)
	assert.Contains(t, entries[0].Content, "key")
	assert.Contains(t, entries[0].Content, "value")

	// Check Text entry
	assert.Equal(t, "TEXT", entries[1].Level)
	assert.Equal(t, "Plain Text Log", entries[1].Msg)
}

func TestPlaybackModel_Init(t *testing.T) {
	m := NewPlaybackModel(nil)
	assert.Nil(t, m.Init())
}

func TestPlaybackModel_Update_Resize(t *testing.T) {
	m := NewPlaybackModel([]LogEntry{{Msg: "Test"}})

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	updated, _ := m.Update(msg)
	newModel := updated.(PlaybackModel)

	assert.Equal(t, 100, newModel.width)
	assert.Equal(t, 50, newModel.height)
	// Check viewport size (height - header 1)
	assert.Equal(t, 49, newModel.viewport.Height)
}

func TestPlaybackModel_Update_Details(t *testing.T) {
	entries := []LogEntry{
		{Msg: "E1", Content: "Details 1"},
	}
	m := NewPlaybackModel(entries)

	// Resize to initialize viewport
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	m = updated.(PlaybackModel)

	// Select first item by default? List usually selects first.
	// We verify selection.
	assert.Equal(t, "E1", m.list.SelectedItem().(LogEntry).Msg)

	// Press Enter
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	newModel := updated.(PlaybackModel)

	assert.True(t, newModel.viewingDetails)

	// Check content in View
	assert.Contains(t, newModel.View(), "Details 1")
	assert.Contains(t, newModel.View(), "Entry Details")

	// Press Esc to close
	updated, _ = newModel.Update(tea.KeyMsg{Type: tea.KeyEsc})
	finalModel := updated.(PlaybackModel)

	assert.False(t, finalModel.viewingDetails)
	assert.NotContains(t, finalModel.View(), "Entry Details")
}

func TestPlaybackModel_Update_Quit(t *testing.T) {
	m := NewPlaybackModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())
}
