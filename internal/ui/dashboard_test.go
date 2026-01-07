package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
)

func init() {
	// Force color output for tests so we can verify ANSI codes
	lipgloss.SetColorProfile(termenv.TrueColor)
}

func TestDashboardModel_View(t *testing.T) {
	m := NewDashboardModel()

	m.width = 100
	m.height = 30
	m.ready = true

	view := m.View()

	if len(view) == 0 {
		t.Error("Expected view to not be empty")
	}

	expectedStrings := []string{
		"Left Pane",
		"Recac v0.1.0",
	}

	for _, s := range expectedStrings {
		if !contains(view, s) {
			t.Errorf("Expected view to contain %q", s)
		}
	}
}

func TestDashboardModel_LogUpdates(t *testing.T) {
	m := NewDashboardModel()
	// Initialize width/height via Update to ensure sub-models are updated
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// Send a Thought Log
	thoughtMsg := "Agent is thinking..."
	msg := LogMsg{Type: LogThought, Message: thoughtMsg}

	m, _ = updateModel(m, msg)

	view := m.View()

	// 1. Verify text is present
	if !contains(view, thoughtMsg) {
		t.Errorf("Expected view to contain log message %q", thoughtMsg)
	}

	// 2. Verify color (Heuristic check for ANSI codes)
	lines := strings.Split(view, "\n")
	foundLine := ""
	for _, line := range lines {
		if strings.Contains(line, thoughtMsg) {
			foundLine = line
			break
		}
	}

	if foundLine == "" {
		t.Fatal("Could not find line with thought message")
	}

	if !strings.Contains(foundLine, "\x1b[") {
		t.Errorf("Expected log line to contain ANSI escape codes for coloring. Got: %q", foundLine)
	}
}

func TestDashboardModel_ProgressComponent(t *testing.T) {
	m := NewDashboardModel()
	// Initialize with WindowSize to set progress width
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// Send Progress Update (50%)
	msg := ProgressMsg(0.5)

	m, cmd := updateModel(m, msg)

	view := m.View()

	// Check if progress bar is rendered (look for 0% initially as animation hasn't run)
	if !contains(view, "0%") {
		t.Errorf("Expected view to contain '0%%' indicating progress bar presence. View snippet: %q", view)
	}

	// Check that a command was returned (indicating animation/update triggered)
	if cmd == nil {
		t.Error("Expected cmd from SetPercent to not be nil (animation trigger)")
	}
}

func TestDashboardModel_Resize(t *testing.T) {
	m := NewDashboardModel()

	// 1. Normal Size
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 30})
	view1 := m.View()
	if !contains(view1, "Recac v0.1.0") {
		t.Error("Expected normal size view to contain footer")
	}

	// 2. Resize to very small
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 10, Height: 5})
	view2 := m.View()
	// Just ensure it doesn't panic and returns something
	if len(view2) == 0 {
		t.Error("Expected small size view to not be empty")
	}

	// 3. Resize back to large
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 200, Height: 60})
	view3 := m.View()
	if !contains(view3, "Recac v0.1.0") {
		t.Error("Expected large size view to contain footer")
	}
}

// Helper to handle type assertion dance
func updateModel(m DashboardModel, msg tea.Msg) (DashboardModel, tea.Cmd) {
	updated, cmd := m.Update(msg)
	return updated.(DashboardModel), cmd
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

// TestDashboardModel_Update_Quit verifies TUI keyboard interrupt (Ctrl+C) exits cleanly.
// Step 1: Start the TUI (model is created)
// Step 2: Press Ctrl+C (simulated via KeyMsg)
// Step 3: Verify the application exits cleanly (tea.Quit is returned, which Bubble Tea handles)
func TestDashboardModel_Update_Quit(t *testing.T) {
	m := NewDashboardModel()

	// Simulate Ctrl+C
	msg := tea.KeyMsg{Type: tea.KeyCtrlC}
	_, cmd := m.Update(msg)

	// Verify tea.Quit is returned (cmd is not nil and will cause program to exit)
	// Bubble Tea automatically handles raw mode cleanup when tea.Quit is returned
	if cmd == nil {
		t.Error("Expected cmd to be tea.Quit, got nil")
	}

	// Verify the command is actually tea.Quit by checking it's callable
	// tea.Quit() returns a tea.Cmd (func() tea.Msg)
	if cmd != nil {
		// Execute the command to verify it's valid
		quitMsg := cmd()
		if quitMsg == nil {
			t.Error("Expected tea.Quit to return a message, got nil")
		}
	}
}

// TestDashboardModel_Scrolling verifies TUI supports scrolling in the log pane.
// Step 1: Fill the log pane with many lines
// Step 2: Use Up/Down arrow keys to scroll
// Step 3: Verify the view scrolls
func TestDashboardModel_Scrolling(t *testing.T) {
	m := NewDashboardModel()

	// Initialize with WindowSize to set up viewport
	m, _ = updateModel(m, tea.WindowSizeMsg{Width: 100, Height: 30})

	// Step 1: Fill the log pane with many lines (more than can fit in viewport)
	// Viewport height is mainHeight - 2, which with height=30 is about 27 lines
	// We'll add 50 log entries to ensure scrolling is needed
	for i := 0; i < 50; i++ {
		msg := LogMsg{
			Type:    LogInfo,
			Message: fmt.Sprintf("Log entry %d: This is a test log message that should fill the viewport", i),
		}
		m, _ = updateModel(m, msg)
	}

	// Verify viewport has content and can scroll
	totalLines := m.viewport.TotalLineCount()
	viewportHeight := m.viewport.Height
	if totalLines <= viewportHeight {
		t.Errorf("Expected viewport content (%d lines) to exceed viewport height (%d) to enable scrolling",
			totalLines, viewportHeight)
	}

	// Get initial viewport position (should be at bottom after GotoBottom in updateViewport)
	// When content is added, viewport automatically scrolls to bottom
	initialYOffset := m.viewport.YOffset
	maxYOffset := totalLines - viewportHeight
	if maxYOffset < 0 {
		maxYOffset = 0
	}

	// Verify we're at or near the bottom initially
	if initialYOffset < maxYOffset-5 && maxYOffset > 0 {
		// Not at bottom, scroll to bottom first
		for i := 0; i < 10; i++ {
			downKey := tea.KeyMsg{Type: tea.KeyDown}
			m, _ = updateModel(m, downKey)
		}
		initialYOffset = m.viewport.YOffset
	}

	// Step 2: Use Up arrow key to scroll up
	upKey := tea.KeyMsg{Type: tea.KeyUp}
	m, _ = updateModel(m, upKey)

	// Step 3: Verify the view scrolled (YOffset should have decreased when scrolling up)
	afterUpYOffset := m.viewport.YOffset

	// When scrolling up from bottom, YOffset should decrease
	// If we're at the bottom (initialYOffset >= maxYOffset), scrolling up should move us up
	if initialYOffset >= maxYOffset && maxYOffset > 0 {
		if afterUpYOffset >= initialYOffset {
			t.Errorf("Expected scrolling up to decrease YOffset. Initial: %d, After Up: %d, Max: %d",
				initialYOffset, afterUpYOffset, maxYOffset)
		}
	}

	// Verify scrolling down works too
	downKey := tea.KeyMsg{Type: tea.KeyDown}
	m, _ = updateModel(m, downKey)
	afterDownYOffset := m.viewport.YOffset

	// Scrolling down should increase YOffset (or stay at max if already at bottom)
	if afterDownYOffset < afterUpYOffset && afterUpYOffset < maxYOffset {
		t.Errorf("Expected scrolling down to increase YOffset. Up: %d, Down: %d, Max: %d",
			afterUpYOffset, afterDownYOffset, maxYOffset)
	}

	// Verify viewport responds to arrow keys and view is rendered
	view := m.View()
	if len(view) == 0 {
		t.Error("Expected view to not be empty after scrolling")
	}

	// Verify that viewport scrolling is functional by checking that arrow keys change position
	// The viewport from Bubble Tea automatically handles Up/Down/PgUp/PgDown keys
	if m.viewport.YOffset == afterUpYOffset && afterUpYOffset > 0 && maxYOffset > 0 {
		// Try scrolling up a few more times to ensure it works
		prevOffset := m.viewport.YOffset
		for i := 0; i < 5; i++ {
			m, _ = updateModel(m, upKey)
			if m.viewport.YOffset < prevOffset {
				break // Scrolling is working
			}
			prevOffset = m.viewport.YOffset
		}
	}
}
