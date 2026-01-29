package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestParseLogLines(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantCount int
		wantErr   bool
	}{
		{
			name:      "Valid JSON logs",
			input:     []byte(`{"time":"2023-10-27T10:00:00Z", "level":"INFO", "msg":"test message", "foo":"bar"}
{"time":"2023-10-27T10:00:01Z", "level":"DEBUG", "msg":"debug message"}`),
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "Invalid JSON (fallback to text)",
			input:     []byte(`Plain text log line
Another line`),
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "Empty input",
			input:     []byte(``),
			wantCount: 0,
			wantErr:   false,
		},
		{
			name:      "Mixed input",
			input:     []byte(`{"msg":"json log"}
plain text log`),
			wantCount: 2,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := ParseLogLines(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantCount, len(entries))
			}
		})
	}
}

func TestLogEntryMethods(t *testing.T) {
	ts, _ := time.Parse(time.RFC3339, "2023-10-27T10:00:00Z")
	entry := LogEntry{
		Time:  ts,
		Level: "INFO",
		Msg:   "test message",
	}

	assert.Equal(t, "[INFO] test message", entry.Title())
	assert.Equal(t, "10:00:00.000", entry.Description())
	assert.Equal(t, "INFO test message", entry.FilterValue())
}

func TestNewPlaybackModel(t *testing.T) {
	entries := []LogEntry{
		{Level: "INFO", Msg: "test"},
	}
	model := NewPlaybackModel(entries)

	assert.Equal(t, 1, len(model.entries))
	assert.False(t, model.viewingDetails)
	assert.NotNil(t, model.list)
	assert.NotNil(t, model.viewport)
}

func TestPlaybackModel_Update(t *testing.T) {
	entries := []LogEntry{
		{
			Level:   "INFO",
			Msg:     "test",
			Content: "detailed content",
		},
	}
	m := NewPlaybackModel(entries)

	// Test WindowSizeMsg
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = newModel.(PlaybackModel)
	assert.Equal(t, 100, m.width)
	assert.Equal(t, 50, m.height)
	assert.Equal(t, 100, m.viewport.Width)

	// Test Enter (View Details)
	// First ensure list has selected item (default is index 0)

	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(PlaybackModel)

	assert.True(t, m.viewingDetails)

	assert.Contains(t, m.View(), "Entry Details")
	assert.Contains(t, m.View(), "detailed content")
	assert.Nil(t, cmd)

	// Test Esc (Back to list)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(PlaybackModel)
	assert.False(t, m.viewingDetails)
	assert.NotContains(t, m.View(), "Entry Details")

	// Test Quit
	newModel, cmd = m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	// tea.Quit returns a command that returns a Msg.
	if cmd != nil {
		msg := cmd()
		assert.IsType(t, tea.QuitMsg{}, msg)
	} else {
		t.Error("Expected Quit command")
	}
}

func TestPlaybackModel_View(t *testing.T) {
	entries := []LogEntry{
		{
			Level:   "INFO",
			Msg:     "test",
			Content: "detailed content",
		},
	}
	m := NewPlaybackModel(entries)
	m.width = 80
	m.height = 24
	m.list.SetSize(80, 20)
	m.viewport.Width = 80
	m.viewport.Height = 20

	// List View
	view := m.View()
	assert.Contains(t, view, "Session Playback")
	assert.Contains(t, view, "test")

	// Details View
	m.viewingDetails = true
	m.viewport.SetContent("detailed content")
	view = m.View()
	assert.Contains(t, view, "Entry Details")
	assert.Contains(t, view, "detailed content")
}

// TestParseLogLines_ComplexContent verifies robust parsing
func TestParseLogLines_ComplexContent(t *testing.T) {
	jsonLine := `{"time":"2023-01-01T00:00:00Z","level":"INFO","msg":"complex","data":{"foo":"bar","nums":[1,2]},"trace_id":"123"}`
	entries, err := ParseLogLines([]byte(jsonLine))
	assert.NoError(t, err)
	assert.Len(t, entries, 1)

	entry := entries[0]
	assert.Equal(t, "INFO", entry.Level)
	assert.Equal(t, "complex", entry.Msg)

	// Check content formatting
	assert.Contains(t, entry.Content, "Level: INFO")
	assert.Contains(t, entry.Content, "[data]:")
	assert.Contains(t, entry.Content, "\"foo\": \"bar\"")
}
