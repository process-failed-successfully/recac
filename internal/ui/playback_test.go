package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestLogEntry_Methods(t *testing.T) {
	now := time.Now()
	entry := LogEntry{
		Time:    now,
		Level:   "INFO",
		Msg:     "Test message",
		Content: "Test content",
	}

	assert.Equal(t, "[INFO] Test message", entry.Title())
	assert.Equal(t, now.Format("15:04:05.000"), entry.Description())
	assert.Equal(t, "INFO Test message", entry.FilterValue())
}

func TestNewPlaybackModel(t *testing.T) {
	entries := []LogEntry{
		{Msg: "Entry 1"},
		{Msg: "Entry 2"},
	}

	model := NewPlaybackModel(entries)

	assert.Equal(t, len(entries), len(model.entries))
	assert.NotNil(t, model.list)
	assert.NotNil(t, model.viewport)
	assert.False(t, model.viewingDetails)
	assert.Equal(t, "Session Playback", model.list.Title)
}

func TestPlaybackModel_Update(t *testing.T) {
	entries := []LogEntry{
		{Msg: "Entry 1", Content: "Details 1"},
	}
	model := NewPlaybackModel(entries)

	// Initialize size
	model, _ = updatePlaybackModel(model, tea.WindowSizeMsg{Width: 100, Height: 50})

	// Test: Select item and press Enter
	// By default first item is selected in list
	model, cmd := updatePlaybackModel(model, tea.KeyMsg{Type: tea.KeyEnter})
	assert.Nil(t, cmd) // No command expected usually, or maybe batch
	assert.True(t, model.viewingDetails)
	assert.Contains(t, model.viewport.View(), "Details 1")

	// Test: Press Esc to go back
	model, cmd = updatePlaybackModel(model, tea.KeyMsg{Type: tea.KeyEsc})
	assert.Nil(t, cmd)
	assert.False(t, model.viewingDetails)

	// Test: Ctrl+C to quit
	_, cmd = updatePlaybackModel(model, tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.NotNil(t, cmd)
	assert.IsType(t, tea.QuitMsg{}, cmd())
}

// Helper to cast tea.Model back to PlaybackModel
func updatePlaybackModel(m PlaybackModel, msg tea.Msg) (PlaybackModel, tea.Cmd) {
	newM, cmd := m.Update(msg)
	return newM.(PlaybackModel), cmd
}

func TestParseLogLines(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{
			name: "Valid JSON",
			input: `{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"Hello"}
{"time":"2023-10-27T10:01:00Z","level":"ERROR","msg":"World"}`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "Mixed Content",
			input: `{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"Hello"}
Not a JSON line
{"time":"2023-10-27T10:01:00Z","level":"ERROR","msg":"World"}`,
			wantLen: 3,
			wantErr: false,
		},
		{
			name:    "Empty Input",
			input:   "",
			wantLen: 0,
			wantErr: false,
		},
		{
			name: "Complex JSON fields",
			input: `{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"Complex","data":{"key":"value"},"items":[1,2]}`,
			wantLen: 1,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := ParseLogLines([]byte(tt.input))
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.wantLen, len(entries))
			}

			if tt.name == "Complex JSON fields" && len(entries) > 0 {
				assert.Contains(t, entries[0].Content, "key")
				assert.Contains(t, entries[0].Content, "value")
			}

			if tt.name == "Mixed Content" && len(entries) > 0 {
				assert.Equal(t, "TEXT", entries[1].Level)
				assert.Equal(t, "Not a JSON line", entries[1].Msg)
			}
		})
	}
}

func TestPlaybackModel_View(t *testing.T) {
	entries := []LogEntry{
		{Msg: "Entry 1", Content: "Details 1"},
	}
	model := NewPlaybackModel(entries)
	// Initialize size to avoid panic or empty view
	model, _ = updatePlaybackModel(model, tea.WindowSizeMsg{Width: 100, Height: 50})

	// List View
	view := model.View()
	assert.Contains(t, view, "Session Playback")
	assert.Contains(t, view, "Entry 1")

	// Detail View
	model.viewingDetails = true
	model.viewport.SetContent("Details 1") // Manually set for test
	view = model.View()
	assert.Contains(t, view, "Entry Details")
	assert.Contains(t, view, "Details 1")
}
