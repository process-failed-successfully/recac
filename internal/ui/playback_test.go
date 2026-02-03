package ui

import (
	"encoding/json"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestLogEntry_Methods(t *testing.T) {
	entry := LogEntry{
		Time:  time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC),
		Level: "INFO",
		Msg:   "Test message",
	}

	assert.Equal(t, "[INFO] Test message", entry.Title())
	assert.Equal(t, "10:00:00.000", entry.Description())
	assert.Equal(t, "INFO Test message", entry.FilterValue())
}

func TestParseLogLines(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{
			name: "Valid JSON log",
			input: `{"time": "2023-10-27T10:00:00Z", "level": "INFO", "msg": "test"}
{"time": "2023-10-27T10:00:01Z", "level": "ERROR", "msg": "fail"}`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "Empty lines",
			input:   "\n\n",
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "Invalid JSON (Text fallback)",
			input: `This is a text log
Another line`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "Mixed JSON and Text",
			input: `{"time": "2023-10-27T10:00:00Z", "level": "INFO", "msg": "test"}
This is a text log`,
			wantLen: 2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseLogLines([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Len(t, got, tt.wantLen)
			}
		})
	}
}

func TestParseLogLines_DetailChecks(t *testing.T) {
	input := `{"time": "2023-10-27T10:00:00Z", "level": "INFO", "msg": "test", "extra": "data"}`
	entries, err := ParseLogLines([]byte(input))
	assert.NoError(t, err)
	assert.Len(t, entries, 1)

	e := entries[0]
	assert.Equal(t, "INFO", e.Level)
	assert.Equal(t, "test", e.Msg)
	assert.Equal(t, "data", e.Raw["extra"])
	assert.Contains(t, e.Content, "extra")
	assert.Contains(t, e.Content, "data")
}

func TestParseLogLines_TextFallback(t *testing.T) {
	input := "Just text"
	entries, err := ParseLogLines([]byte(input))
	assert.NoError(t, err)
	assert.Len(t, entries, 1)

	e := entries[0]
	assert.Equal(t, "TEXT", e.Level)
	assert.Equal(t, "Just text", e.Msg)
	assert.Equal(t, "Just text", e.Content)
}

func TestNewPlaybackModel(t *testing.T) {
	entries := []LogEntry{
		{Level: "INFO", Msg: "One"},
		{Level: "ERROR", Msg: "Two"},
	}

	model := NewPlaybackModel(entries)
	assert.Len(t, model.entries, 2)
	// list items are populated
	assert.Equal(t, 2, len(model.list.Items()))
	assert.False(t, model.viewingDetails)
}

func TestNewPlaybackModel_Empty(t *testing.T) {
	model := NewPlaybackModel([]LogEntry{})
	assert.Len(t, model.entries, 0)
	assert.Equal(t, 0, len(model.list.Items()))
}

// Complex nesting test
func TestParseLogLines_ComplexJSON(t *testing.T) {
	data := map[string]interface{}{
		"time":  "2023-10-27T10:00:00Z",
		"level": "DEBUG",
		"msg":   "complex",
		"obj": map[string]interface{}{
			"foo": "bar",
		},
		"arr": []int{1, 2, 3},
	}
	bytesData, _ := json.Marshal(data)
	entries, err := ParseLogLines(bytesData)
	assert.NoError(t, err)
	assert.Len(t, entries, 1)

	e := entries[0]
	assert.Contains(t, e.Content, `"foo": "bar"`)
	assert.Contains(t, e.Content, "[")
}

func TestPlaybackModel_Update_View(t *testing.T) {
	entries := []LogEntry{
		{Level: "INFO", Msg: "Test Log", Content: "Detailed content"},
	}
	m := NewPlaybackModel(entries)

	// Send resize msg to initialize list/viewport size
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updatedModel.(PlaybackModel)

	// Test Initial View (List)
	view := m.View()
	assert.Contains(t, view, "Test Log")
	assert.NotContains(t, view, "Detailed content")

	// Select item and press Enter
	m.list.Select(0)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedModel.(PlaybackModel)

	assert.True(t, m.viewingDetails)
	viewDetails := m.View()
	assert.Contains(t, viewDetails, "Detailed content")
	assert.Contains(t, viewDetails, "Entry Details")

	// Scroll viewport (coverage)
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	m = updatedModel.(PlaybackModel)

	// Press Esc to go back
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedModel.(PlaybackModel)
	assert.False(t, m.viewingDetails)

	// Quit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())

	// Resize
	updatedModel, _ = m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = updatedModel.(PlaybackModel)
	assert.Equal(t, 100, m.width)
}
