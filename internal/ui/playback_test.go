package ui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseLogLines(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantLen int
		wantErr bool
	}{
		{
			name: "valid json logs",
			input: `{"time":"2023-10-26T10:00:00Z","level":"INFO","msg":"test message","key":"value"}
{"time":"2023-10-26T10:01:00Z","level":"ERROR","msg":"error message"}`,
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "empty input",
			input:   "",
			wantLen: 0,
			wantErr: false,
		},
		{
			name:    "mixed content",
			input:   `{"level":"INFO","msg":"json log"}
raw log line
`,
			wantLen: 2,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := ParseLogLines([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogLines() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(entries) != tt.wantLen {
				t.Errorf("ParseLogLines() got %d entries, want %d", len(entries), tt.wantLen)
			}
		})
	}
}

func TestPlaybackModel(t *testing.T) {
	entries := []LogEntry{
		{
			Time:  time.Now(),
			Level: "INFO",
			Msg:   "test",
			Raw:   map[string]interface{}{"foo": "bar"},
		},
	}

	m := NewPlaybackModel(entries)

	// Test Update: Resize (Set size first to ensure list renders)
	m, _ = updatePlayback(m, tea.WindowSizeMsg{Width: 100, Height: 50})
	if m.width != 100 || m.height != 50 {
		t.Error("Resize failed")
	}

	// Test Init
	if m.Init() != nil {
		t.Error("Init should return nil")
	}

	// Test View (List)
	view := m.View()
	if !strings.Contains(view, "test") {
		t.Error("View should contain log message")
	}

	// Test Update: Enter details
	m, _ = updatePlayback(m, tea.KeyMsg{Type: tea.KeyEnter})
	if !m.viewingDetails {
		t.Error("Enter should switch to details view")
	}

	// Test View (Details)
	view = m.View()
	if !strings.Contains(view, "Entry Details") {
		t.Error("Details view should contain header")
	}

	// Test Update: Esc details
	m, _ = updatePlayback(m, tea.KeyMsg{Type: tea.KeyEsc})
	if m.viewingDetails {
		t.Error("Esc should switch back to list view")
	}
}

// Helper to cast model
func updatePlayback(m PlaybackModel, msg tea.Msg) (PlaybackModel, tea.Cmd) {
	mod, cmd := m.Update(msg)
	return mod.(PlaybackModel), cmd
}

func TestLogEntryMethods(t *testing.T) {
	e := LogEntry{
		Time:  time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC),
		Level: "INFO",
		Msg:   "hello",
	}

	if e.Title() != "[INFO] hello" {
		t.Errorf("Title got %s", e.Title())
	}
	if e.Description() != "12:00:00.000" {
		t.Errorf("Description got %s", e.Description())
	}
	if e.FilterValue() != "INFO hello" {
		t.Errorf("FilterValue got %s", e.FilterValue())
	}
}
