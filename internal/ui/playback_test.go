package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseLogLines(t *testing.T) {
	// 1. JSON Log
	logLine := `{"time":"2023-01-01T12:00:00Z","level":"INFO","msg":"Test message","key":"value"}`
	entries, err := ParseLogLines([]byte(logLine))
	if err != nil {
		t.Fatalf("Failed to parse log line: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Level != "INFO" {
		t.Errorf("Expected Level INFO, got %s", e.Level)
	}
	if e.Msg != "Test message" {
		t.Errorf("Expected Msg 'Test message', got %s", e.Msg)
	}
	expectedTime, _ := time.Parse(time.RFC3339, "2023-01-01T12:00:00Z")
	if !e.Time.Equal(expectedTime) {
		t.Errorf("Expected time %v, got %v", expectedTime, e.Time)
	}

	// 2. Plain Text Log
	plainText := "Just a plain log line"
	entries, err = ParseLogLines([]byte(plainText))
	if err != nil {
		t.Fatalf("Failed to parse plain text: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(entries))
	}
	if entries[0].Level != "TEXT" {
		t.Errorf("Expected Level TEXT, got %s", entries[0].Level)
	}
}

func TestPlaybackModel_Update(t *testing.T) {
	entries := []LogEntry{
		{
			Time:    time.Now(),
			Level:   "INFO",
			Msg:     "Test",
			Content: "Details...",
		},
	}
	m := NewPlaybackModel(entries)

	// 1. Init
	if m.Init() != nil {
		t.Error("Init should return nil")
	}

	// 2. Resize
	updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m = updatedM.(PlaybackModel)
	if m.width != 100 || m.height != 40 {
		t.Error("Resize failed")
	}

	// 3. Select Item (Enter)
	// List starts with first item selected by default.
	updatedM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedM.(PlaybackModel)
	if !m.viewingDetails {
		t.Error("Expected viewingDetails to be true after Enter")
	}
	if m.View() == "" {
		t.Error("View should not be empty")
	}

	// 4. Go Back (Esc)
	updatedM, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedM.(PlaybackModel)
	if m.viewingDetails {
		t.Error("Expected viewingDetails to be false after Esc")
	}

	// 5. Quit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil { // Actually cmd might be Tea.Quit which is internal, usually returns a Msg
		// tea.Quit is a Cmd.
	}
}

func TestLogEntry_Methods(t *testing.T) {
	e := LogEntry{
		Time:  time.Now(),
		Level: "INFO",
		Msg:   "Test",
	}
	if e.Title() != "[INFO] Test" {
		t.Errorf("Title mismatch: %s", e.Title())
	}
	if e.FilterValue() != "INFO Test" {
		t.Errorf("FilterValue mismatch: %s", e.FilterValue())
	}
	if e.Description() == "" {
		t.Error("Description should not be empty")
	}
}
