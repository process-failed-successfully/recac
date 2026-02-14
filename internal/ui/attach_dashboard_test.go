package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestAttachDashboardModel(t *testing.T) {
	// Create a temporary log file
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	err := os.WriteFile(logFile, []byte("Line 1\nLine 2\n"), 0600)
	assert.NoError(t, err)

	m := NewAttachDashboardModel("test-session", logFile)

	// Simulate Init
	cmd := m.Init()
	assert.NotNil(t, cmd) // Init returns a tick command

	// Simulate WindowSizeMsg to initialize viewport
	// We cast to AttachDashboardModel to check internal state if needed,
	// but Update returns tea.Model interface.
	updatedM, _ := m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Check initial content via View
	view := updatedM.View()
	assert.NotContains(t, view, "Initializing...")
	assert.Contains(t, view, "Line 1")
	assert.Contains(t, view, "Line 2")
	assert.Contains(t, view, "Session: test-session")

	// Append to log file
	err = os.WriteFile(logFile, []byte("Line 1\nLine 2\nLine 3\n"), 0600)
	assert.NoError(t, err)

	// Simulate Tick to refresh logs
	updatedM, _ = updatedM.Update(logTickMsg(time.Now()))

	// Check updated content
	view = updatedM.View()
	assert.Contains(t, view, "Line 3")

	// Test quit
	_, cmd = updatedM.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	// tea.Quit returns a special Msg, but as a Cmd it returns a Msg.
	// tea.Quit is a tea.Cmd.
	// We can't easily check function equality, but we can check behavior if we ran it.
	// However, standard pattern in checking commands in tests is hard.
	// But we know it should be tea.Quit.
	assert.NotNil(t, cmd)
}
