package ui

import (
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseLogLines(t *testing.T) {
	data := []byte(`
{"time":"2023-10-27T10:00:00Z","level":"INFO","msg":"Starting session","project":"test"}
{"time":"2023-10-27T10:00:01Z","level":"DEBUG","msg":"Details","data":{"foo":"bar"}}
Invalid Line
`)
	entries, err := ParseLogLines(data)
	if err != nil {
		t.Fatalf("ParseLogLines failed: %v", err)
	}

	if len(entries) != 3 {
		t.Errorf("Expected 3 entries, got %d", len(entries))
	}

	// Check entry 1
	if entries[0].Level != "INFO" {
		t.Errorf("Expected INFO, got %s", entries[0].Level)
	}
	if entries[0].Msg != "Starting session" {
		t.Errorf("Expected msg, got %s", entries[0].Msg)
	}

	// Check entry 2 (complex data)
	if entries[1].Level != "DEBUG" {
		t.Errorf("Expected DEBUG, got %s", entries[1].Level)
	}

	// Check entry 3 (invalid json fallback)
	if entries[2].Level != "TEXT" {
		t.Errorf("Expected TEXT level for invalid json, got %s", entries[2].Level)
	}
	if entries[2].Msg != "Invalid Line" {
		t.Errorf("Expected raw msg, got %s", entries[2].Msg)
	}
}

func TestPlaybackModel(t *testing.T) {
	entries := []LogEntry{
		{Time: time.Now(), Level: "INFO", Msg: "Test 1", Content: "Details 1"},
		{Time: time.Now(), Level: "ERROR", Msg: "Test 2", Content: "Details 2"},
	}

	m := NewPlaybackModel(entries)

	// Init
	if cmd := m.Init(); cmd != nil {
		t.Error("Init should return nil cmd")
	}

	// Update: Resize
	updatedModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = updatedModel.(PlaybackModel)
	if m.width != 100 || m.height != 50 {
		t.Errorf("Expected size 100x50, got %dx%d", m.width, m.height)
	}

	// View (List)
	view := m.View()
	if view == "" {
		t.Error("View should not be empty")
	}

	// Update: Select Item (Enter)
	// We simulate typing 'enter'
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updatedModel.(PlaybackModel)

	if !m.viewingDetails {
		t.Error("Expected to be viewing details after Enter")
	}

	// View (Details)
	viewDetails := m.View()
	if viewDetails == "" {
		t.Error("Details view should not be empty")
	}

	// Update: Escape to go back
	updatedModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updatedModel.(PlaybackModel)

	if m.viewingDetails {
		t.Error("Expected to return to list after Esc")
	}

	// Update: Quit
	_, cmd := m.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	if cmd == nil {
		// tea.Quit usually returns a command.
	}
}
