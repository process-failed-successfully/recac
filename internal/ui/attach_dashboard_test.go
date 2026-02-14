package ui

import (
	"os"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachDashboardModel_ReadNewLogs(t *testing.T) {
	// Create temporary log file
	tmpFile, err := os.CreateTemp("", "recac-test-*.log")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	// Initialize model
	model := NewAttachDashboardModel(tmpFile.Name())

	// Test Init
	cmd := model.Init()
	assert.NotNil(t, cmd)

	// Test Initial Update (tick)
	updatedModel, cmd := model.Update(attachTickMsg(time.Now()))
	assert.NotNil(t, cmd) // Should schedule read + next tick

	m, ok := updatedModel.(AttachDashboardModel)
	require.True(t, ok)
	assert.Equal(t, "", m.content)

	// Write content to file
	content := "Hello, World!\n"
	_, err = tmpFile.WriteString(content)
	require.NoError(t, err)

	// Manually verify readLogsCmd logic works
    // Since readLogsCmd is not exported, we can't call it directly if we were outside the package.
    // But we are in package `ui` (same package), so we can call `readLogsCmd`.

    readCmd := readLogsCmd(tmpFile.Name(), 0)
    msg := readCmd()

    readMsg, ok := msg.(readLogsMsg)
    require.True(t, ok)
    assert.Equal(t, content, readMsg.content)
    assert.NoError(t, readMsg.err)
    assert.Equal(t, int64(len(content)), readMsg.offset)

	// Test Update with readLogsMsg
	updatedModel, cmd = model.Update(readMsg)
	m, ok = updatedModel.(AttachDashboardModel)
	require.True(t, ok)
	assert.Equal(t, content, m.content)
    assert.Equal(t, int64(len(content)), m.offset)

    // Write more content
    moreContent := "More logs...\n"
    _, err = tmpFile.WriteString(moreContent)
    require.NoError(t, err)

    // Read again with updated offset
    readCmd = readLogsCmd(tmpFile.Name(), m.offset)
    msg = readCmd()
    readMsg, ok = msg.(readLogsMsg)
    require.True(t, ok)
    assert.Equal(t, moreContent, readMsg.content)

    // Update model again
    updatedModel, cmd = m.Update(readMsg)
    m, ok = updatedModel.(AttachDashboardModel)
    require.True(t, ok)
    assert.Equal(t, content+moreContent, m.content)
}

func TestAttachDashboardModel_WindowSize(t *testing.T) {
    model := NewAttachDashboardModel("dummy.log")

    // Set initial content
    model.content = "Some logs"

    // Simulate window size msg
    updatedModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
    m, ok := updatedModel.(AttachDashboardModel)
    require.True(t, ok)

    assert.True(t, m.ready)
    assert.Equal(t, 100, m.viewport.Width)
    // Height should be less than 50 due to margins
    assert.Less(t, m.viewport.Height, 50)
}
