package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestBlameModel_Update_Navigation(t *testing.T) {
	lines := []BlameLine{
		{Content: "Line 1"},
		{Content: "Line 2"},
		{Content: "Line 3"},
	}
	m := NewBlameModel(lines, nil, nil)
	m.Height = 10
	m.Width = 20

	// Initial State
	assert.Equal(t, 0, m.Cursor)

	// Move Down
	msg := tea.KeyMsg{Type: tea.KeyDown}
	newM, _ := m.Update(msg)
	m = newM.(BlameModel)
	assert.Equal(t, 1, m.Cursor)

	// Move Down again
	newM, _ = m.Update(msg)
	m = newM.(BlameModel)
	assert.Equal(t, 2, m.Cursor)

	// Move Down at bottom (should stay)
	newM, _ = m.Update(msg)
	m = newM.(BlameModel)
	assert.Equal(t, 2, m.Cursor)

	// Move Up
	msg = tea.KeyMsg{Type: tea.KeyUp}
	newM, _ = m.Update(msg)
	m = newM.(BlameModel)
	assert.Equal(t, 1, m.Cursor)
}

func TestBlameModel_Update_Scrolling(t *testing.T) {
	// Create enough lines to scroll
	lines := make([]BlameLine, 20)
	for i := 0; i < 20; i++ {
		lines[i] = BlameLine{Content: "Line"}
	}

	m := NewBlameModel(lines, nil, nil)
	m.Height = 10 // Visible content height approx 5 (10 - 5 header/footer)
	// Actually contentHeight function logic: Height - 5. So 5 lines visible.

	// Move down 10 times
	for i := 0; i < 10; i++ {
		msg := tea.KeyMsg{Type: tea.KeyDown}
		newM, _ := m.Update(msg)
		m = newM.(BlameModel)
	}

	assert.Equal(t, 10, m.Cursor)
	// ViewportStart should have moved.
	// Visible lines = 5.
	// Cursor at 10. Start should be at least 10 - 5 + 1 = 6.
	assert.Greater(t, m.ViewportStart, 0)
}

func TestBlameModel_Update_Details(t *testing.T) {
	lines := []BlameLine{{SHA: "abc", Content: "Line 1"}}

	called := false
	fetchDiff := func(sha string) (string, error) {
		called = true
		assert.Equal(t, "abc", sha)
		return "diff content", nil
	}

	m := NewBlameModel(lines, fetchDiff, nil)
	m.Height = 20
	m.Width = 40
	m.detailsViewport.Width = 40
	m.detailsViewport.Height = 20

	// Press Enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	newM, cmd := m.Update(msg)
	m = newM.(BlameModel)

	// Cmd should not be nil
	assert.NotNil(t, cmd)

	// Execute cmd to verify callback is called?
	// In Bubble Tea, we can't easily run the cmd in unit test without a program loop,
	// but we can assert that a command was returned.
	// To actually run it:
	if cmd != nil {
		// This is a thunk that returns a Msg
		resMsg := cmd()
		// We expect a blameDiffMsg
		dMsg, ok := resMsg.(blameDiffMsg)
		assert.True(t, ok)
		assert.Equal(t, "diff content", dMsg.content)
	}
	assert.True(t, called)

	// Now send the blameDiffMsg back to Update
	dMsg := blameDiffMsg{content: "diff content", err: nil}
	newM, _ = m.Update(dMsg)
	m = newM.(BlameModel)

	assert.True(t, m.viewingDetails)
	assert.Contains(t, m.detailsViewport.View(), "diff content")

	// Escape to close
	escMsg := tea.KeyMsg{Type: tea.KeyEsc}
	newM, _ = m.Update(escMsg)
	m = newM.(BlameModel)
	assert.False(t, m.viewingDetails)
}
