package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseLogLines(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantErr   bool
	}{
		{
			name: "valid jsonl",
			input: `{"time":"2023-10-26T10:00:00Z","level":"INFO","msg":"test message","foo":"bar"}
{"time":"2023-10-26T10:00:01Z","level":"ERROR","msg":"error message"}`,
			wantCount: 2,
			wantErr:   false,
		},
		{
			name:      "empty input",
			input:     "",
			wantCount: 0,
			wantErr:   false,
		},
		{
			name: "mixed content",
			input: `{"time":"2023-10-26T10:00:00Z","level":"INFO","msg":"json message"}
plain text message
{"level":"WARN","msg":"another json"}`,
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "empty lines",
			input:     "\n\n",
			wantCount: 0,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := ParseLogLines([]byte(tt.input))
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseLogLines() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(entries) != tt.wantCount {
				t.Errorf("ParseLogLines() count = %d, want %d", len(entries), tt.wantCount)
			}
		})
	}
}

func TestLogEntry_Methods(t *testing.T) {
	now := time.Now()
	entry := LogEntry{
		Time:  now,
		Level: "INFO",
		Msg:   "Test Message",
	}

	if title := entry.Title(); title != "[INFO] Test Message" {
		t.Errorf("Title() = %v, want %v", title, "[INFO] Test Message")
	}

	if desc := entry.Description(); desc != now.Format("15:04:05.000") {
		t.Errorf("Description() = %v, want %v", desc, now.Format("15:04:05.000"))
	}

	if filter := entry.FilterValue(); filter != "INFO Test Message" {
		t.Errorf("FilterValue() = %v, want %v", filter, "INFO Test Message")
	}
}

func TestPlaybackModel_Update(t *testing.T) {
	entries := []LogEntry{
		{Time: time.Now(), Level: "INFO", Msg: "Line 1", Content: "Details 1"},
		{Time: time.Now(), Level: "ERROR", Msg: "Line 2", Content: "Details 2"},
	}
	// Initial model is PlaybackModel struct
	initialModel := NewPlaybackModel(entries)
	var model tea.Model = initialModel

	// Test Resize
	var cmd tea.Cmd
	model, cmd = model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	_ = cmd // ignore cmd

	m := model.(PlaybackModel)
	if m.width != 100 || m.height != 50 {
		t.Errorf("Window resize failed: got %dx%d, want 100x50", m.width, m.height)
	}

	// Test Enter (View Details)
	// Select first item
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = model.(PlaybackModel)
	if !m.viewingDetails {
		t.Error("Enter should switch to viewing details")
	}
	if !strings.Contains(m.View(), "Details 1") {
		t.Error("View should contain details content")
	}

	// Test Esc (Back to List)
	model, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = model.(PlaybackModel)
	if m.viewingDetails {
		t.Error("Esc should switch back to list view")
	}
	if !strings.Contains(m.View(), "Line 1") {
		t.Error("View should contain list content")
	}

	// Test Ctrl+C (Quit)
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		t.Error("Ctrl+C should return a Quit command")
	}
}

func TestPlaybackModel_ComplexContent(t *testing.T) {
	rawJSON := `{"nested":{"key":"value"},"array":[1,2,3]}`
	entries, err := ParseLogLines([]byte(rawJSON))
	if err != nil {
		t.Fatalf("Failed to parse: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}

	entry := entries[0]
	if !strings.Contains(entry.Content, `"key": "value"`) {
		t.Error("Content should contain pretty printed map")
	}
	if !strings.Contains(entry.Content, `[`) {
		t.Error("Content should contain pretty printed array")
	}
}

func TestPlaybackModel_LiveUpdate(t *testing.T) {
	// Create temp dir and file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "test.log")

	// Write initial content
	initialContent := `{"msg":"Line 1"}` + "\n"
	err := os.WriteFile(logPath, []byte(initialContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write initial content: %v", err)
	}

	// Initialize model
	entries, _ := ParseLogLines([]byte(initialContent))
	model := NewPlaybackModel(entries)
	model = model.WithLiveMode(logPath, int64(len(initialContent)))

	// Check initial state
	if len(model.list.Items()) != 1 {
		t.Errorf("Initial count = %d, want 1", len(model.list.Items()))
	}

	// Append new content
	f, err := os.OpenFile(logPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		t.Fatalf("Failed to open file for append: %v", err)
	}

	newContent := `{"msg":"Line 2"}` + "\n"
	if _, err := f.WriteString(newContent); err != nil {
		t.Fatalf("Failed to append content: %v", err)
	}
	f.Close()

	// Trigger update manually by sending the tick message
	// Since playbackTickMsg is unexported (but in same package), we can use it.
	msg := playbackTickMsg(time.Now())

	updatedModel, _ := model.Update(msg)
	m := updatedModel.(PlaybackModel)

	// Check if new item is added
	if len(m.list.Items()) != 2 {
		t.Errorf("Updated count = %d, want 2", len(m.list.Items()))
	}

	// Check content of second item
	item := m.list.Items()[1].(LogEntry)
	if item.Msg != "Line 2" {
		t.Errorf("Second item msg = %s, want Line 2", item.Msg)
	}
}
