package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestLogEntry(t *testing.T) {
	entry := LogEntry{
		Time:  time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Level: "INFO",
		Msg:   "Test Message",
	}

	assert.Equal(t, "[INFO] Test Message", entry.Title())
	assert.Equal(t, "12:00:00.000", entry.Description())
	assert.Equal(t, "INFO Test Message", entry.FilterValue())
}

func TestParseLogLines(t *testing.T) {
	// JSON Line
	jsonLine := `{"time":"2023-01-01T12:00:00Z","level":"INFO","msg":"Json Message","key":"value"}`
	// Text Line
	textLine := `Plain Text Message`
	// Empty Line
	emptyLine := `   `

	data := []byte(jsonLine + "\n" + textLine + "\n" + emptyLine)

	entries, err := ParseLogLines(data)
	assert.NoError(t, err)
	assert.Len(t, entries, 2)

	// Check JSON Entry
	assert.Equal(t, "INFO", entries[0].Level)
	assert.Equal(t, "Json Message", entries[0].Msg)
	assert.Equal(t, "value", entries[0].Raw["key"])
	assert.Contains(t, entries[0].Content, "key")
	assert.Contains(t, entries[0].Content, "value")

	// Check Text Entry
	assert.Equal(t, "TEXT", entries[1].Level)
	assert.Equal(t, "Plain Text Message", entries[1].Msg)
}

func TestPlaybackModel(t *testing.T) {
	entries := []LogEntry{
		{Msg: "Entry 1", Content: "Details 1"},
		{Msg: "Entry 2", Content: "Details 2"},
	}

	m := NewPlaybackModel(entries)
	assert.NotNil(t, m)

	// Init
	assert.Nil(t, m.Init())

	// Resize
	newM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	pm := newM.(PlaybackModel)
	assert.Equal(t, 100, pm.width)
	assert.Equal(t, 50, pm.height)

	// Select Item (Enter)
	newM, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm = newM.(PlaybackModel)
	assert.True(t, pm.viewingDetails)
	assert.Contains(t, pm.View(), "Details 1")

	// Scroll details (down)
	newM, _ = pm.Update(tea.KeyMsg{Type: tea.KeyDown}) // Viewport update

	// Exit details (Esc)
	newM, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pm = newM.(PlaybackModel)
	assert.False(t, pm.viewingDetails)

	// Quit (Ctrl+C)
	_, cmd := pm.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())
}
