package ui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestAttachDashboard_UpdatesFromLogFile(t *testing.T) {
	// 1. Create temp file
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "session.log")
	f, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()

	// 2. Initialize model
	model := NewAttachDashboardModel("test-session", logPath)

	// Simulate WindowSizeMsg to initialize viewport
	updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m := updatedModel.(attachDashboardModel)

	if !m.ready {
		t.Error("Model should be ready after WindowSizeMsg")
	}

	// 3. Write to file
	logContent := "Line 1\nLine 2\n"
	if _, err := f.WriteString(logContent); err != nil {
		t.Fatal(err)
	}
	// Sync to ensure it's on disk
	f.Sync()

	// 4. Send tickMsg
	updatedModel, _ = m.Update(attachTickMsg(time.Now()))
	m = updatedModel.(attachDashboardModel)

	// 5. Verify content
	if m.content != logContent {
		t.Errorf("Expected content %q, got %q", logContent, m.content)
	}

	// Note: Viewport view might contain ANSI codes or partial lines depending on height
	// We check if it contains at least one of the lines
	view := m.viewport.View()
	if !strings.Contains(view, "Line 1") {
		t.Errorf("Viewport view does not contain 'Line 1'. View: %q", view)
	}

	// 6. Append more content
	moreContent := "Line 3\n"
	if _, err := f.WriteString(moreContent); err != nil {
		t.Fatal(err)
	}
	f.Sync()

	updatedModel, _ = m.Update(attachTickMsg(time.Now()))
	m = updatedModel.(attachDashboardModel)

	expected := logContent + moreContent
	if m.content != expected {
		t.Errorf("Expected content %q, got %q", expected, m.content)
	}
}
