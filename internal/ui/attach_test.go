package ui

import (
	"os"
	"testing"

	"github.com/charmbracelet/bubbles/viewport"
)

func TestAttachDashboardModel_Update_FileUpdate(t *testing.T) {
	// Setup temp file
	tmpfile, err := os.CreateTemp("", "attach_test_*.log")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpfile.Name()) // clean up

	// Initialize model
	model := NewAttachDashboardModel("test-session", tmpfile.Name(), "Running")
	model.ready = true
	model.viewport = viewport.New(80, 20)

	// write some initial content
	msg1 := `{"level":"INFO","msg":"Hello","time":"2024-01-01T00:00:00Z"}` + "\n"
	if _, err := tmpfile.Write([]byte(msg1)); err != nil {
		t.Fatal(err)
	}

	// simulate file update msg
	msg := fileUpdateMsg{
		newContent: []byte(msg1),
		newOffset:  int64(len(msg1)),
	}

	// Update model
	updatedModel, _ := model.Update(msg)
	m := updatedModel.(AttachDashboardModel)

	// Verify entries
	if len(m.entries) != 1 {
		t.Errorf("Expected 1 entry, got %d", len(m.entries))
	} else {
		if m.entries[0].Msg != "Hello" {
			t.Errorf("Expected msg 'Hello', got '%s'", m.entries[0].Msg)
		}
	}

	// Verify offset
	if m.offset != int64(len(msg1)) {
		t.Errorf("Expected offset %d, got %d", len(msg1), m.offset)
	}

	// Write more content
	msg2Str := `{"level":"WARN","msg":"Warning","time":"2024-01-01T00:00:01Z"}` + "\n"
	if _, err := tmpfile.Write([]byte(msg2Str)); err != nil {
		t.Fatal(err)
	}

	// simulate next update
	msg2 := fileUpdateMsg{
		newContent: []byte(msg2Str),
		newOffset:  int64(len(msg1) + len(msg2Str)),
	}

	updatedModel, _ = m.Update(msg2)
	m = updatedModel.(AttachDashboardModel)

	if len(m.entries) != 2 {
		t.Errorf("Expected 2 entries, got %d", len(m.entries))
	} else {
		if m.entries[1].Msg != "Warning" {
			t.Errorf("Expected msg 'Warning', got '%s'", m.entries[1].Msg)
		}
	}
}

func TestAttachDashboardModel_ViewportUpdate(t *testing.T) {
	// Dummy content
	entries := []LogEntry{
		{Content: "Line 1"},
		{Content: "Line 2"},
	}

	model := NewAttachDashboardModel("test", "dummy", "Running")
	model.ready = true
	model.viewport = viewport.New(100, 10)
	model.entries = entries

	model.updateViewportContent()

	content := model.viewport.View()
	if content == "" {
		t.Error("Expected content in viewport, got empty string")
	}
}
