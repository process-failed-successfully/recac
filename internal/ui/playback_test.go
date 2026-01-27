package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestParseLogLines(t *testing.T) {
	// 1. Valid JSON
	jsonLog := `{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"test message","extra":"value"}`
	entries, err := ParseLogLines([]byte(jsonLog))
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "INFO", entries[0].Level)
	assert.Equal(t, "test message", entries[0].Msg)
	assert.Equal(t, "value", entries[0].Raw["extra"])
	assert.WithinDuration(t, time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC), entries[0].Time, time.Second)

	// 2. Invalid JSON (Raw Text)
	rawLog := "Just a raw log line"
	entries, err = ParseLogLines([]byte(rawLog))
	assert.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "TEXT", entries[0].Level)
	assert.Equal(t, rawLog, entries[0].Msg)

	// 3. Empty Line
	mixedLog := "line1\n\nline2"
	entries, err = ParseLogLines([]byte(mixedLog))
	assert.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, "line1", entries[0].Msg)
	assert.Equal(t, "line2", entries[1].Msg)
}

func TestPlaybackModel_Init(t *testing.T) {
	entries := []LogEntry{
		{Time: time.Now(), Level: "INFO", Msg: "test"},
	}
	m := NewPlaybackModel(entries)
	assert.NotNil(t, m.list)
	assert.NotNil(t, m.viewport)
	assert.Len(t, m.entries, 1)
	assert.Nil(t, m.Init())
}

func TestPlaybackModel_Update_Resize(t *testing.T) {
	m := NewPlaybackModel([]LogEntry{{Msg: "test"}})

	newModel, cmd := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	updated := newModel.(PlaybackModel)

	assert.Nil(t, cmd) // Batch cmd might be nil or not, usually nil for resize
	assert.Equal(t, 100, updated.width)
	assert.Equal(t, 50, updated.height)
	// List height should be adjusted (Height - 1)
	// List width should be 100
}

func TestPlaybackModel_Update_Details(t *testing.T) {
	entries := []LogEntry{
		{Msg: "test", Content: "Detailed Content"},
	}
	m := NewPlaybackModel(entries)
	// Set size to ensure viewport renders content
	m.viewport.Width = 20
	m.viewport.Height = 10

	// Select first item
	m.list.Select(0)

	// 1. Enter to view details
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	updated := newModel.(PlaybackModel)
	assert.True(t, updated.viewingDetails)
	assert.Contains(t, updated.viewport.View(), "Detailed Content")

	// 2. Esc to exit details
	newModel, _ = updated.Update(tea.KeyMsg{Type: tea.KeyEsc})
	updated = newModel.(PlaybackModel)
	assert.False(t, updated.viewingDetails)
}

func TestPlaybackModel_View(t *testing.T) {
	m := NewPlaybackModel([]LogEntry{{Msg: "test", Level: "INFO", Time: time.Now()}})
	// Set size to ensure list renders
	m.list.SetSize(40, 20)
	m.viewport.Width = 40
	m.viewport.Height = 20
	m.width = 40
	m.height = 20

	// List View
	view := m.View()
	assert.Contains(t, view, "test")

	// Details View
	m.viewingDetails = true
	m.viewport.SetContent("details")
	view = m.View()
	assert.Contains(t, view, "Entry Details")
	assert.Contains(t, view, "details")
}

func TestPlaybackModel_Update_Quit(t *testing.T) {
	m := NewPlaybackModel([]LogEntry{})
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())
}
