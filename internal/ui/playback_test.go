package ui

import (
	"encoding/json"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestParseLogLines(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		verify    func(*testing.T, []LogEntry)
	}{
		{
			name: "valid json lines",
			input: `{"time":"2023-10-01T12:00:00Z", "level":"INFO", "msg":"test msg", "foo":"bar"}
{"time":"2023-10-01T12:00:01Z", "level":"ERROR", "msg":"error msg"}`,
			wantCount: 2,
			verify: func(t *testing.T, entries []LogEntry) {
				assert.Equal(t, "INFO", entries[0].Level)
				assert.Equal(t, "test msg", entries[0].Msg)
				assert.Equal(t, "bar", entries[0].Raw["foo"])

				assert.Equal(t, "ERROR", entries[1].Level)
				assert.Equal(t, "error msg", entries[1].Msg)
			},
		},
		{
			name:      "raw text line",
			input:     "This is just a raw log line",
			wantCount: 1,
			verify: func(t *testing.T, entries []LogEntry) {
				assert.Equal(t, "TEXT", entries[0].Level)
				assert.Equal(t, "This is just a raw log line", entries[0].Msg)
				assert.Equal(t, "This is just a raw log line", entries[0].Content)
			},
		},
		{
			name: "mixed content",
			input: `{"time":"2023-10-01T12:00:00Z", "level":"INFO", "msg":"json line"}
Raw line here`,
			wantCount: 2,
			verify: func(t *testing.T, entries []LogEntry) {
				assert.Equal(t, "INFO", entries[0].Level)
				assert.Equal(t, "TEXT", entries[1].Level)
			},
		},
		{
			name:      "empty input",
			input:     "",
			wantCount: 0,
			verify:    nil,
		},
		{
			name:      "empty lines",
			input:     "\n   \n",
			wantCount: 0,
			verify:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := ParseLogLines([]byte(tt.input))
			assert.NoError(t, err)
			assert.Len(t, entries, tt.wantCount)
			if tt.verify != nil {
				tt.verify(t, entries)
			}
		})
	}
}

func TestLogEntry_Methods(t *testing.T) {
	entry := LogEntry{
		Time:  time.Date(2023, 10, 1, 12, 0, 0, 0, time.UTC),
		Level: "INFO",
		Msg:   "Test message",
	}

	assert.Equal(t, "[INFO] Test message", entry.Title())
	assert.Equal(t, "12:00:00.000", entry.Description())
	assert.Equal(t, "INFO Test message", entry.FilterValue())
}

func TestPlaybackModel_Init(t *testing.T) {
	entries := []LogEntry{
		{Msg: "test 1", Content: "details 1"},
		{Msg: "test 2", Content: "details 2"},
	}
	m := NewPlaybackModel(entries)

	assert.Nil(t, m.Init())
	assert.Equal(t, 2, len(m.entries))
	assert.False(t, m.viewingDetails)
}

func TestPlaybackModel_Update_Resize(t *testing.T) {
	m := NewPlaybackModel([]LogEntry{{Msg: "test"}})

	msg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newM, _ := m.Update(msg)
	finalM := newM.(PlaybackModel)

	assert.Equal(t, 100, finalM.width)
	assert.Equal(t, 50, finalM.height)
	// Header takes 1 line, so list/viewport height should be 49
	assert.Equal(t, 49, finalM.viewport.Height)
}

func TestPlaybackModel_Update_Navigation(t *testing.T) {
	entries := []LogEntry{
		{Msg: "test 1", Content: "details 1"},
	}
	m := NewPlaybackModel(entries)
	// Ensure list has size so selection works (though New sets items)
	// Set initial size
	m, _ = updatePlaybackModel(m, tea.WindowSizeMsg{Width: 80, Height: 20})

	// 1. Enter to view details
	// Select item first (default 0)
	m, _ = updatePlaybackModel(m, tea.KeyMsg{Type: tea.KeyEnter})

	if !m.viewingDetails {
		t.Error("Should be viewing details after Enter")
	}
	if m.viewport.View() == "" {
		t.Error("Viewport should have content")
	}

	// 2. Esc to go back
	m, _ = updatePlaybackModel(m, tea.KeyMsg{Type: tea.KeyEsc})

	if m.viewingDetails {
		t.Error("Should not be viewing details after Esc")
	}

	// 3. 'q' to go back (if in details)
	m.viewingDetails = true
	m, _ = updatePlaybackModel(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if m.viewingDetails {
		t.Error("Should not be viewing details after q")
	}
}

func TestPlaybackModel_Update_Quit(t *testing.T) {
	m := NewPlaybackModel(nil)
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())
}

func TestPlaybackModel_View(t *testing.T) {
	entries := []LogEntry{
		{Msg: "Visible Item", Content: "Secret Details"},
	}
	m := NewPlaybackModel(entries)
	m, _ = updatePlaybackModel(m, tea.WindowSizeMsg{Width: 80, Height: 20})

	// List View
	view := m.View()
	assert.Contains(t, view, "Visible Item")
	assert.NotContains(t, view, "Secret Details")

	// Details View
	m.viewingDetails = true
	m.viewport.SetContent("Secret Details") // Manually set as Update does it
	view = m.View()
	assert.Contains(t, view, "Entry Details")
	assert.Contains(t, view, "Secret Details")
}

func TestParseLogLines_ComplexValues(t *testing.T) {
	rawJSON := map[string]interface{}{
		"time": "2023-01-01T00:00:00Z",
		"msg": "complex",
		"obj": map[string]interface{}{"a": 1},
		"arr": []interface{}{1, 2},
	}
	bytes, _ := json.Marshal(rawJSON)

	entries, err := ParseLogLines(bytes)
	assert.NoError(t, err)
	assert.Len(t, entries, 1)

	content := entries[0].Content
	assert.Contains(t, content, `"a": 1`)
	assert.Contains(t, content, "[\n  1,\n  2\n]")
}

// Helper
func updatePlaybackModel(m PlaybackModel, msg tea.Msg) (PlaybackModel, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(PlaybackModel), cmd
}
