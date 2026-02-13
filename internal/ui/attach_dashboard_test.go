package ui

import (
	"bufio"
	"strings"
	"testing"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
)

func TestAttachDashboardModel_Init(t *testing.T) {
	m := NewAttachDashboardModel("test-session", "test.log")
	// Mock reader
	m.reader = bufio.NewReader(strings.NewReader("test"))

	cmd := m.Init()
	assert.NotNil(t, cmd)
}

func TestAttachDashboardModel_Update_Logs(t *testing.T) {
	m := NewAttachDashboardModel("test-session", "test.log")
	m.ready = true
	// Initialize viewport to avoid panic
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	// Mock reader for subsequent calls
	m.reader = bufio.NewReader(strings.NewReader("next line\n"))

	// Simulate receiving a log line
	logLine := "INFO: Agent started\n"
	newModel, cmd := m.Update(logMsg(logLine))

	updatedM := newModel.(*attachDashboardModel)

	assert.Contains(t, updatedM.logContent.String(), "INFO: Agent started")
	assert.NotNil(t, cmd, "Should return a command to wait for next log")
}

func TestAttachDashboardModel_Update_Status(t *testing.T) {
	m := NewAttachDashboardModel("test-session", "test.log")

	session := &runner.SessionState{Name: "test-session", Status: "running"}
	state := &agent.State{
		Model: "gpt-4",
		TokenUsage: agent.TokenUsage{TotalTokens: 500},
	}

	msg := statusRefreshedMsg{
		session: session,
		agentState: state,
		gitDiffStat: "2 files changed",
	}

	newModel, _ := m.Update(msg)
	updatedM := newModel.(*attachDashboardModel)

	assert.Equal(t, session, updatedM.session)
	assert.Equal(t, state, updatedM.agentState)
	assert.Equal(t, "2 files changed", updatedM.gitDiffStat)
}

func TestAttachDashboardModel_View(t *testing.T) {
	m := NewAttachDashboardModel("test-session", "test.log")
	m.ready = true
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})

	m.session = &runner.SessionState{
		Name: "test-session",
		Status: "running",
		StartTime: time.Now(),
		Goal: "Fix bugs",
	}
	m.agentState = &agent.State{
		Model: "gpt-4",
	}
	m.logContent.WriteString("Log Line 1\nLog Line 2\n")
	// Update viewport content manually as View() reads from it
	m.viewport.SetContent(m.logContent.String())

	output := m.View()

	assert.Contains(t, output, "RECAC Session: test-session")
	assert.Contains(t, output, "running") // Status
	assert.Contains(t, output, "Fix bugs") // Goal
	assert.Contains(t, output, "Log Line 1")
	assert.Contains(t, output, "Log Line 2")
}

func TestAttachDashboardModel_WaitForLog(t *testing.T) {
	// Test the tea.Cmd logic directly?
	// It's a bit tricky because it involves sleep loops.
	// We can test reading a line immediately.

	input := "immediate line\n"
	reader := bufio.NewReader(strings.NewReader(input))

	cmd := waitForLog(reader)
	msg := cmd()

	assert.IsType(t, logMsg(""), msg)
	assert.Equal(t, "immediate line\n", string(msg.(logMsg)))
}
