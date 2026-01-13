package ui

import (
	"errors"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSessionLoader provides a controllable session loader for tests.
func mockSessionLoader(sessions []Session, err error) SessionLoader {
	return func() ([]Session, error) {
		return sessions, err
	}
}

func TestDashboardModel_SuccessfulLoad(t *testing.T) {
	// --- Setup ---
	mockSessions := []Session{
		{Name: "session-1", Status: "running", Location: "local", StartTime: "10:00:00", Cost: "$0.1234", Details: "Details for session 1"},
		{Name: "session-2", Status: "completed", Location: "k8s", StartTime: "11:00:00", Cost: "$0.5678", Details: "Details for session 2"},
	}
	SetSessionLoader(mockSessionLoader(mockSessions, nil))

	// --- Execute ---
	m := NewDashboardModel()

	// Initial view should be loading
	assert.Contains(t, m.View(), "Loading sessions...")

	// Initialize the model and get the command to load sessions
	initCmd := m.Init()
	require.NotNil(t, initCmd)

	// Execute the command to get the loaded message
	msg := initCmd()
	loadedMsg, ok := msg.(sessionsLoadedMsg)
	require.True(t, ok, "Init command should produce a sessionsLoadedMsg")

	// Update the model with the loaded data
	model, _ := m.Update(loadedMsg)
	m = model.(DashboardModel)

	// --- Assert ---
	view := m.View()
	assert.NotContains(t, view, "Loading sessions...")
	assert.Contains(t, view, "RECAC Sessions")
	assert.Contains(t, view, "session-1")
	assert.Contains(t, view, "session-2")
	// Initially, details for the first item should NOT be visible until selected
	assert.Contains(t, view, "Select a session to see details.")

	// --- Test Selection ---
	// Simulate pressing the down arrow to select the second item
	downKey := tea.KeyMsg{Type: tea.KeyDown}
	model, _ = m.Update(downKey)
	m = model.(DashboardModel)

	// Now the view should contain the details of the second session
	view = m.View()
	assert.Contains(t, view, "Details for session 2")
	assert.NotContains(t, view, "Details for session 1")
}

func TestDashboardModel_ErrorOnLoad(t *testing.T) {
	// --- Setup ---
	expectedErr := errors.New("failed to connect to daemon")
	SetSessionLoader(mockSessionLoader(nil, expectedErr))

	// --- Execute ---
	m := NewDashboardModel()
	initCmd := m.Init()
	msg := initCmd() // Execute the command to get the message
	model, _ := m.Update(msg)
	m = model.(DashboardModel)

	// --- Assert ---
	view := m.View()
	assert.NotContains(t, view, "Loading sessions...")
	assert.Contains(t, view, "Error loading sessions: failed to connect to daemon")
}

func TestDashboardModel_Quit(t *testing.T) {
	// --- Setup ---
	SetSessionLoader(mockSessionLoader(nil, nil))
	m := NewDashboardModel()

	// --- Execute ---
	// First, load the sessions to get out of the loading state
	initCmd := m.Init()
	msg := initCmd()
	model, _ := m.Update(msg)

	// Then, send a quit message
	_, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")})

	// --- Assert ---
	// Note: We cannot directly compare tea.Cmd functions. We check if it's non-nil.
	// A more robust test would involve a message channel.
	assert.NotNil(t, cmd, "Expected a command on 'q' keypress")

	// Also test Ctrl+C
	_, cmd = model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
	assert.NotNil(t, cmd, "Expected a command on 'ctrl+c' keypress")
}
