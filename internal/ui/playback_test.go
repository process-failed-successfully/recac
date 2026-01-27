package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestNewPlaybackModel(t *testing.T) {
	entries := []LogEntry{
		{
			Time:  time.Now(),
			Level: "INFO",
			Msg:   "Test message",
		},
	}
	model := NewPlaybackModel(entries)

	// Set size to ensure title is rendered
	m, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	model = m.(PlaybackModel)

	// Access entries via private field using reflection?
	// Or just verify public behavior via View/Update.
	// We can check list title
	assert.Contains(t, model.View(), "Session Playback")
}

func TestPlaybackModel_Update(t *testing.T) {
	entries := []LogEntry{
		{
			Time:    time.Now(),
			Level:   "INFO",
			Msg:     "Test message",
			Content: "Detailed content",
		},
	}
	model := NewPlaybackModel(entries)

	// 1. Resize
	m, cmd := model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	assert.Nil(t, cmd)
	model = m.(PlaybackModel)
	// Assertions on size would require access to private fields or checking View output

	// 2. Select item (Enter)
	// Need to make sure item is selected. List usually selects first item by default.
	m, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd)
	model = m.(PlaybackModel)

	// Should be viewing details now
	assert.Contains(t, model.View(), "Entry Details")
	assert.Contains(t, model.View(), "Detailed content")

	// 3. Exit details (Esc)
	m, cmd = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, cmd)
	model = m.(PlaybackModel)

	// Should be back to list
	assert.Contains(t, model.View(), "Session Playback")

	// 4. Quit (Ctrl+C)
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}

func TestParseLogLines(t *testing.T) {
	data := []byte(`{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"Hello","foo":"bar"}
Invalid JSON line
`)
	entries, err := ParseLogLines(data)
	assert.NoError(t, err)
	assert.Len(t, entries, 2)

	// Check valid entry
	assert.Equal(t, "INFO", entries[0].Level)
	assert.Equal(t, "Hello", entries[0].Msg)
	assert.Contains(t, entries[0].Content, "foo")
	assert.Contains(t, entries[0].Content, "bar")

	// Check invalid entry
	assert.Equal(t, "TEXT", entries[1].Level)
	assert.Equal(t, "Invalid JSON line", entries[1].Msg)
}
