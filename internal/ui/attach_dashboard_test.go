package ui

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAttachDashboard_Init(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	err := os.WriteFile(logFile, []byte("initial logs\n"), 0644)
	require.NoError(t, err)

	model := NewAttachDashboardModel("test-session", logFile, "running", 1234)
	cmd := model.Init()
	assert.NotNil(t, cmd)
}

func TestAttachDashboard_ReadLogsCmd(t *testing.T) {
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")
	content := []byte("hello world\n")
	err := os.WriteFile(logFile, content, 0644)
	require.NoError(t, err)

	// Test initial read
	cmd := readLogsCmd(logFile, 0)
	msg := cmd()

	readMsg, ok := msg.(logReadMsg)
	require.True(t, ok)
	assert.NoError(t, readMsg.err)
	assert.Equal(t, string(content), readMsg.content)
	assert.Equal(t, int64(len(content)), readMsg.offset)

	// Test incremental read
	newContent := []byte("new line\n")
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	_, err = f.Write(newContent)
	require.NoError(t, err)
	f.Close()

	cmd = readLogsCmd(logFile, int64(len(content)))
	msg = cmd()

	readMsg, ok = msg.(logReadMsg)
	require.True(t, ok)
	assert.NoError(t, readMsg.err)
	assert.Equal(t, string(newContent), readMsg.content)
	assert.Equal(t, int64(len(content)+len(newContent)), readMsg.offset)
}

func TestAttachDashboard_Update(t *testing.T) {
	// Setup
	tmpDir := t.TempDir()
	logFile := filepath.Join(tmpDir, "test.log")

	model := NewAttachDashboardModel("test-session", logFile, "running", 1234)
	model.ready = true
	model.width = 80
	model.height = 24

	// Test WindowSizeMsg
	resizeMsg := tea.WindowSizeMsg{Width: 100, Height: 30}
	updatedModel, _ := model.Update(resizeMsg)
	m := updatedModel.(AttachDashboardModel)
	assert.Equal(t, 100, m.width)
	assert.Equal(t, 30, m.height)
	assert.Equal(t, 100, m.viewport.Width)

	// Test logReadMsg
	logMsg := logReadMsg{
		content: "new logs\n",
		offset:  10,
		err:     nil,
	}
	updatedModel, cmd := model.Update(logMsg)
	m = updatedModel.(AttachDashboardModel)

	assert.Equal(t, "new logs\n", m.content)
	assert.Equal(t, int64(10), m.offset)
	assert.Nil(t, cmd) // Should be nil as we append to batch usually, but here just updating state

	// Test attachTickMsg
	tickMsg := attachTickMsg(time.Now())
	_, cmd = model.Update(tickMsg)
	assert.NotNil(t, cmd) // Should return batch command (tick + read)
}
