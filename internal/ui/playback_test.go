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
		wantErr bool
	}{
		{
			name:    "Valid JSON",
			input:   `{"time":"2023-10-26T10:00:00Z", "level":"INFO", "msg":"Test message"}`,
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "Multiple Lines",
			input: `{"time":"2023-10-26T10:00:00Z", "level":"INFO", "msg":"Line 1"}
{"time":"2023-10-26T10:00:01Z", "level":"ERROR", "msg":"Line 2"}`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "Invalid JSON (Text Fallback)",
			input:   `Just some raw text log line`,
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "Empty Lines",
			input:   "\n  \n",
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "Mixed Content",
			input:   `{"msg":"json"}
text line`,
			wantLen: 2,
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
			}
			assert.Equal(t, tt.wantLen, len(entries))

			if tt.wantLen > 0 {
				// Basic check on first entry
				assert.NotEmpty(t, entries[0].Content)
				if tt.name == "Valid JSON" {
					assert.Equal(t, "INFO", entries[0].Level)
					assert.Equal(t, "Test message", entries[0].Msg)
				}
				if tt.name == "Invalid JSON (Text Fallback)" {
					assert.Equal(t, "TEXT", entries[0].Level)
					assert.Equal(t, "Just some raw text log line", entries[0].Msg)
				}
			}
		})
	}
}

func TestLogEntryMethods(t *testing.T) {
	entry := LogEntry{
		Time:  time.Date(2023, 10, 26, 12, 0, 0, 0, time.UTC),
		Level: "INFO",
		Msg:   "Test",
	}

	assert.Equal(t, "[INFO] Test", entry.Title())
	assert.Equal(t, "12:00:00.000", entry.Description())
	assert.Equal(t, "INFO Test", entry.FilterValue())
}

func TestPlaybackModel_Update(t *testing.T) {
	entries := []LogEntry{
		{
			Time:    time.Now(),
			Level:   "INFO",
			Msg:     "Test 1",
			Content: "Details 1",
		},
		{
			Time:    time.Now(),
			Level:   "ERROR",
			Msg:     "Test 2",
			Content: "Details 2",
		},
	}

	m := NewPlaybackModel(entries)

	// Test Init
	assert.Nil(t, m.Init())

	// Test Window Resize
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	pm := newModel.(PlaybackModel)
	assert.Equal(t, 100, pm.width)
	assert.Equal(t, 50, pm.height)

	// Test Enter (View Details)
	// First select an item (default is 0)
	newModel, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEnter})
	pm = newModel.(PlaybackModel)
	assert.True(t, pm.viewingDetails)
	assert.Contains(t, pm.View(), "Entry Details")
	assert.Contains(t, pm.View(), "Details 1")

	// Test Esc (Back to List)
	newModel, _ = pm.Update(tea.KeyMsg{Type: tea.KeyEsc})
	pm = newModel.(PlaybackModel)
	assert.False(t, pm.viewingDetails)
	assert.Contains(t, pm.View(), "Session Playback")

	// Test Quit
	_, cmd := pm.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.Equal(t, tea.Quit(), cmd())
}

func TestParseLogLines_Complex(t *testing.T) {
	input := `{"time":"2023-10-26T10:00:00Z", "level":"INFO", "msg":"Complex", "extra":{"foo":"bar"}, "arr":[1,2,3]}`
	entries, err := ParseLogLines([]byte(input))
	assert.NoError(t, err)
	assert.Len(t, entries, 1)

	entry := entries[0]
	assert.Contains(t, entry.Content, "foo")
	assert.Contains(t, entry.Content, "bar")
	assert.Contains(t, entry.Content, "arr")
}
