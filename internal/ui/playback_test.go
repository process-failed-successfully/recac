package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestParseLogLines(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		check   func(*testing.T, []LogEntry)
	}{
		{
			name: "valid json",
			input: `{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"test message","key":"value"}
{"time":"2023-10-27T10:01:00Z","level":"ERROR","msg":"error message"}`,
			wantLen: 2,
			check: func(t *testing.T, entries []LogEntry) {
				assert.Equal(t, "INFO", entries[0].Level)
				assert.Equal(t, "test message", entries[0].Msg)
				assert.Equal(t, "value", entries[0].Raw["key"])
				assert.Equal(t, "ERROR", entries[1].Level)
			},
		},
		{
			name:    "plain text",
			input:   "Just a random log line\nAnother line",
			wantLen: 2,
			check: func(t *testing.T, entries []LogEntry) {
				assert.Equal(t, "TEXT", entries[0].Level)
				assert.Equal(t, "Just a random log line", entries[0].Msg)
				assert.Equal(t, "TEXT", entries[1].Level)
			},
		},
		{
			name: "mixed content",
			input: `{"level":"INFO","msg":"json log"}
plain text log`,
			wantLen: 2,
			check: func(t *testing.T, entries []LogEntry) {
				assert.Equal(t, "INFO", entries[0].Level)
				assert.Equal(t, "TEXT", entries[1].Level)
			},
		},
		{
			name:    "empty lines",
			input:   "\n\n",
			wantLen: 0,
			check:   nil,
		},
		{
			name: "malformed time",
			input: `{"time":"invalid-time","level":"INFO","msg":"msg"}`,
			wantLen: 1,
			check: func(t *testing.T, entries []LogEntry) {
				// Should fallback or handle gracefully (zero time or ignored)
				assert.Equal(t, "INFO", entries[0].Level)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := ParseLogLines([]byte(tt.input))
			assert.NoError(t, err)
			assert.Equal(t, tt.wantLen, len(entries))
			if tt.check != nil {
				tt.check(t, entries)
			}
		})
	}
}

func TestPlaybackModel_New(t *testing.T) {
	entries := []LogEntry{
		{Level: "INFO", Msg: "test", Time: time.Now()},
	}
	m := NewPlaybackModel(entries)
	assert.Equal(t, 1, len(m.entries))
	assert.NotNil(t, m.list)
	assert.NotNil(t, m.viewport)
	assert.Equal(t, "Session Playback", m.list.Title)
}

func TestPlaybackModel_Init(t *testing.T) {
	m := PlaybackModel{}
	cmd := m.Init()
	assert.Nil(t, cmd)
}

func TestPlaybackModel_Update(t *testing.T) {
	entries := []LogEntry{
		{Level: "INFO", Msg: "msg1", Content: "details1", Time: time.Now()},
		{Level: "ERROR", Msg: "msg2", Content: "details2", Time: time.Now()},
	}
	m := NewPlaybackModel(entries)

	// Window resize
	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 50}
	newM, _ := m.Update(resizeMsg)
	m = newM.(PlaybackModel)
	assert.Equal(t, 100, m.width)
	assert.Equal(t, 50, m.height)
	assert.Equal(t, 100, m.viewport.Width)
	// Check list size logic if needed, but integration with list bubble is internal

	// Select item (Enter)
	// First, ensure an item is selected (list usually selects first by default)
	enterMsg := tea.KeyMsg{Type: tea.KeyEnter}
	newM, _ = m.Update(enterMsg)
	m = newM.(PlaybackModel)
	assert.True(t, m.viewingDetails)
	assert.Contains(t, m.viewport.View(), "details1")

	// Esc to go back
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newM, _ = m.Update(escMsg)
	m = newM.(PlaybackModel)
	assert.False(t, m.viewingDetails)

	// Quit (Ctrl+C)
	quitMsg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(quitMsg)
	assert.Equal(t, tea.Quit(), cmd())
}

func TestPlaybackModel_View(t *testing.T) {
	entries := []LogEntry{
		{Level: "INFO", Msg: "msg1", Content: "details1", Time: time.Now()},
	}
	m := NewPlaybackModel(entries)

	// Simulate resize to ensure viewport has dimensions
	m.width = 100
	m.height = 20
	m.list.SetSize(100, 20)
	m.viewport.Width = 100
	m.viewport.Height = 20

	// List View
	view := m.View()
	assert.Contains(t, view, "msg1")
	assert.Contains(t, view, "INFO")

	// Details View
	m.viewingDetails = true
	m.viewport.SetContent("details1")
	view = m.View()
	assert.Contains(t, view, "Entry Details")
	assert.Contains(t, view, "details1")
}

func TestLogEntry_Methods(t *testing.T) {
	tm := time.Date(2023, 10, 27, 10, 0, 0, 0, time.UTC)
	entry := LogEntry{
		Level: "INFO",
		Msg:   "test",
		Time:  tm,
	}

	assert.Equal(t, "[INFO] test", entry.Title())
	assert.Equal(t, "10:00:00.000", entry.Description())
	assert.Equal(t, "INFO test", entry.FilterValue())
}
