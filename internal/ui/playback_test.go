package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestParseLogLines(t *testing.T) {
	data := []byte(`{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"Test message","key":"value"}
Not JSON line
{"time":"2023-10-27T10:01:00Z","level":"ERROR","msg":"Error occurred"}`)

	entries, err := ParseLogLines(data)
	assert.NoError(t, err)
	assert.Len(t, entries, 3)

	// Check JSON entry
	assert.Equal(t, "INFO", entries[0].Level)
	assert.Equal(t, "Test message", entries[0].Msg)
	assert.Equal(t, "value", entries[0].Raw["key"])
	assert.Contains(t, entries[0].Content, "key")
	assert.Contains(t, entries[0].Content, "value")

	// Check Text entry
	assert.Equal(t, "TEXT", entries[1].Level)
	assert.Equal(t, "Not JSON line", entries[1].Msg)

	// Check List Item methods
	assert.Contains(t, entries[0].Title(), "[INFO] Test message")
	assert.Equal(t, "10:00:00.000", entries[0].Description())
	assert.Equal(t, "INFO Test message", entries[0].FilterValue())
}

func TestPlaybackModel_Update(t *testing.T) {
	entries := []LogEntry{
		{Time: time.Now(), Level: "INFO", Msg: "Test", Content: "Details content"},
	}
	m := NewPlaybackModel(entries)

	// Init
	assert.Nil(t, m.Init())

	// Window Size
	m, _ = updatePlaybackModel(m, tea.WindowSizeMsg{Width: 100, Height: 50})
	assert.Equal(t, 100, m.width)

	// View List (default)
	view := m.View()
	assert.Contains(t, view, "Session Playback")
	assert.Contains(t, view, "Test")

	// Enter to see details
	m, _ = updatePlaybackModel(m, tea.KeyMsg{Type: tea.KeyEnter})
	assert.True(t, m.viewingDetails)

	// View Details
	view = m.View()
	assert.Contains(t, view, "Entry Details")
	assert.Contains(t, view, "Details content")

	// Esc to go back
	m, _ = updatePlaybackModel(m, tea.KeyMsg{Type: tea.KeyEsc})
	assert.False(t, m.viewingDetails)

	// Ctrl+C to quit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.QuitMsg{}, cmd())
}

// Helper to cast model back to PlaybackModel
func updatePlaybackModel(m PlaybackModel, msg tea.Msg) (PlaybackModel, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(PlaybackModel), cmd
}
