package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestParseLogLines(t *testing.T) {
	t.Run("Valid JSON", func(t *testing.T) {
		input := `{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"Test message","key":"value"}
{"time":"2023-10-27T10:00:01.123456Z","level":"ERROR","msg":"Error message"}
{"msg":"No Level"}`
		entries, err := ParseLogLines([]byte(input))
		assert.NoError(t, err)
		assert.Len(t, entries, 3)
		assert.Equal(t, "INFO", entries[0].Level)
		assert.Equal(t, "Test message", entries[0].Msg)
		assert.Equal(t, "ERROR", entries[1].Level)
		assert.Equal(t, "INFO", entries[2].Level) // Default
	})

	t.Run("Invalid JSON", func(t *testing.T) {
		input := `Not a JSON line`
		entries, err := ParseLogLines([]byte(input))
		assert.NoError(t, err) // Should handle gracefully
		assert.Len(t, entries, 1)
		assert.Equal(t, "TEXT", entries[0].Level)
		assert.Equal(t, "Not a JSON line", entries[0].Msg)
	})

	t.Run("Mixed Content", func(t *testing.T) {
		input := `{"level":"INFO","msg":"Valid"}
Invalid`
		entries, err := ParseLogLines([]byte(input))
		assert.NoError(t, err)
		assert.Len(t, entries, 2)
		assert.Equal(t, "INFO", entries[0].Level)
		assert.Equal(t, "TEXT", entries[1].Level)
	})

	t.Run("Complex JSON fields", func(t *testing.T) {
		input := `{"level":"INFO","msg":"Complex","data":{"foo":"bar"},"list":[1,2]}`
		entries, err := ParseLogLines([]byte(input))
		assert.NoError(t, err)
		assert.Len(t, entries, 1)
		assert.Contains(t, entries[0].Content, "foo")
		assert.Contains(t, entries[0].Content, "bar")
	})
}

func TestLogEntryMethods(t *testing.T) {
	tm, _ := time.Parse(time.RFC3339, "2023-10-27T10:00:00Z")
	entry := LogEntry{
		Time:  tm,
		Level: "INFO",
		Msg:   "Test",
	}

	assert.Equal(t, "[INFO] Test", entry.Title())
	assert.Equal(t, "10:00:00.000", entry.Description())
	assert.Equal(t, "INFO Test", entry.FilterValue())
}

func TestPlaybackModel(t *testing.T) {
	entries := []LogEntry{
		{Level: "INFO", Msg: "One"},
		{Level: "ERROR", Msg: "Two"},
	}
	m := NewPlaybackModel(entries)

	t.Run("Init", func(t *testing.T) {
		cmd := m.Init()
		assert.Nil(t, cmd)
	})

	t.Run("Update Resize", func(t *testing.T) {
		msg := tea.WindowSizeMsg{Width: 100, Height: 50}
		updatedModel, _ := m.Update(msg)
		newM := updatedModel.(PlaybackModel)
		assert.Equal(t, 100, newM.width)
		assert.Equal(t, 50, newM.height)
	})

	t.Run("Update Quit", func(t *testing.T) {
		msg := tea.KeyMsg{Type: tea.KeyCtrlC}
		_, cmd := m.Update(msg)
		assert.Equal(t, tea.Quit(), cmd())
	})

	t.Run("Navigation and Details", func(t *testing.T) {
		// Set size first to ensure list is ready
		updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
		m = updatedM.(PlaybackModel)

		// Initial state
		assert.False(t, m.viewingDetails)

		// Enter details
		msg := tea.KeyMsg{Type: tea.KeyEnter}
		updatedModel, _ := m.Update(msg)
		newM := updatedModel.(PlaybackModel)
		assert.True(t, newM.viewingDetails)

		// View Details
		view := newM.View()
		assert.Contains(t, view, "Entry Details")

		// Exit details (Esc)
		msg = tea.KeyMsg{Type: tea.KeyEsc}
		updatedModel, _ = newM.Update(msg)
		newM = updatedModel.(PlaybackModel)
		assert.False(t, newM.viewingDetails)

		// Enter details again
		msg = tea.KeyMsg{Type: tea.KeyEnter}
		updatedModel, _ = newM.Update(msg)
		newM = updatedModel.(PlaybackModel)
		assert.True(t, newM.viewingDetails)

		// Exit details (q)
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}}
		updatedModel, _ = newM.Update(msg)
		newM = updatedModel.(PlaybackModel)
		assert.False(t, newM.viewingDetails)

		// View List
		view = newM.View()
		assert.Contains(t, view, "Session Playback")
	})
}
