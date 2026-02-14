package ui

import (
	"os"
	"recac/internal/runner"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestAttachDashboardModel_Update_Logs(t *testing.T) {
	// Create temp log file
	tmpFile, err := os.CreateTemp("", "test-log-*.log")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	logPath := tmpFile.Name()
	initialContent := "Line 1\nLine 2\n"
	_, err = tmpFile.WriteString(initialContent)
	assert.NoError(t, err)
	tmpFile.Close()

	// Mock session fetcher
	fetchSession := func() (*runner.SessionState, error) {
		return &runner.SessionState{
			Name:   "test-session",
			Status: "running",
			PID:    1234,
		}, nil
	}

	model := NewAttachDashboard("test-session", logPath, fetchSession)

	// Simulate WindowSize to init viewport
	model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// 1. Initial Update (Tick) - starts reading logs
	newModel, _ := model.Update(attachTickMsg(time.Now()))
	m := newModel.(AttachDashboardModel)

	assert.True(t, m.readingLogs)

	// 2. Simulate Log Read Completion
	logMsg := logReadMsg{
		content: initialContent,
		newOffset: int64(len(initialContent)),
		err: nil,
	}
	newModel, _ = m.Update(logMsg)
	m = newModel.(AttachDashboardModel)

	assert.Equal(t, initialContent, m.content)
	assert.Equal(t, int64(len(initialContent)), m.offset)
	assert.False(t, m.readingLogs)

	// 3. Simulate Session Update
	sessionState, _ := fetchSession()
	newModel, _ = m.Update(sessionState)
	m = newModel.(AttachDashboardModel)
	assert.Equal(t, "running", m.session.Status)
}

func TestReadLogsCmd(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-log-cmd-*.log")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	content := "Hello World\n"
	tmpFile.WriteString(content)
	tmpFile.Close()

	// Test reading from 0
	cmd := readLogsCmd(tmpFile.Name(), 0)
	msg := cmd()
	logMsg := msg.(logReadMsg)

	assert.NoError(t, logMsg.err)
	assert.Equal(t, content, logMsg.content)
	assert.Equal(t, int64(len(content)), logMsg.newOffset)

	// Test reading new content
	f, _ := os.OpenFile(tmpFile.Name(), os.O_APPEND|os.O_WRONLY, 0644)
	f.WriteString("Line 2")
	f.Close()

	cmd = readLogsCmd(tmpFile.Name(), int64(len(content)))
	msg = cmd()
	logMsg = msg.(logReadMsg)

	assert.NoError(t, logMsg.err)
	assert.Equal(t, "Line 2", logMsg.content)
	assert.Equal(t, int64(len(content)+len("Line 2")), logMsg.newOffset)
}

func TestAttachDashboardModel_AutoScroll(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-log-scroll-*.log")
	assert.NoError(t, err)
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	fetchSession := func() (*runner.SessionState, error) {
		return &runner.SessionState{Name: "test"}, nil
	}

	model := NewAttachDashboard("test-session", tmpFile.Name(), fetchSession)
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 80, Height: 10})
	m := newModel.(AttachDashboardModel)

	// Generate content
	content := ""
	for i := 0; i < 20; i++ {
		content += "line\n"
	}

	// Feed content via logReadMsg
	logMsg := logReadMsg{
		content: content,
		newOffset: int64(len(content)),
	}
	newModel, _ = m.Update(logMsg)
	m = newModel.(AttachDashboardModel)

	// Verify auto-scroll
	assert.True(t, m.viewport.YOffset > 0, "Viewport should have scrolled down")
	assert.True(t, m.autoScroll)

	// Disable auto-scroll
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	m = newModel.(AttachDashboardModel)
	assert.False(t, m.autoScroll)
}
