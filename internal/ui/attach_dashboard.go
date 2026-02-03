package ui

import (
	"fmt"
	"io"
	"os"
	"recac/internal/runner"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type attachTickMsg time.Time
type logUpdateMsg []LogEntry

// AttachDashboardModel is the TUI model for attaching to a session.
type AttachDashboardModel struct {
	sm          runner.ISessionManager
	sessionName string
	logPath     string
	file        *os.File
	viewport    viewport.Model
	entries     []LogEntry
	status      string
	goal        string
	width       int
	height      int
	autoScroll  bool
}

// NewAttachDashboardModel creates a new model.
func NewAttachDashboardModel(sessionName string, sm runner.ISessionManager) (AttachDashboardModel, error) {
	logPath, err := sm.GetSessionLogs(sessionName)
	if err != nil {
		return AttachDashboardModel{}, fmt.Errorf("failed to get session logs: %w", err)
	}

	file, err := os.Open(logPath)
	if err != nil {
		return AttachDashboardModel{}, fmt.Errorf("failed to open log file: %w", err)
	}

	// Seek to end - 10KB to avoid reading huge files initially
	stat, _ := file.Stat()
	size := stat.Size()
	start := int64(0)
	if size > 10000 {
		start = size - 10000
	}
	file.Seek(start, 0)

	session, err := sm.LoadSession(sessionName)
	status := "Unknown"
	goal := ""
	if err == nil {
		status = session.Status
		goal = session.Goal
	}

	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)

	return AttachDashboardModel{
		sm:          sm,
		sessionName: sessionName,
		logPath:     logPath,
		file:        file,
		status:      status,
		goal:        goal,
		autoScroll:  true,
		viewport:    vp,
	}, nil
}

func (m AttachDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}),
		readLogsCmd(m.file),
	)
}

func (m AttachDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		headerHeight := 4
		m.viewport.Width = msg.Width
		m.viewport.Height = msg.Height - headerHeight
		if m.autoScroll {
			m.viewport.GotoBottom()
		}

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "s":
			m.autoScroll = !m.autoScroll
		default:
			m.viewport, cmd = m.viewport.Update(msg)
			if m.viewport.AtBottom() {
				m.autoScroll = true
			} else {
				m.autoScroll = false
			}
			cmds = append(cmds, cmd)
		}

	case attachTickMsg:
		// Refresh session status
		session, err := m.sm.LoadSession(m.sessionName)
		if err == nil {
			m.status = session.Status
		}
		// Read new logs
		cmds = append(cmds, readLogsCmd(m.file))
		cmds = append(cmds, tea.Tick(time.Millisecond*500, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}))

	case logUpdateMsg:
		if len(msg) > 0 {
			m.entries = append(m.entries, msg...)
			// Limit to last 2000 lines to avoid memory bloat
			if len(m.entries) > 2000 {
				m.entries = m.entries[len(m.entries)-2000:]
			}
			content := renderEntries(m.entries)
			m.viewport.SetContent(content)
			if m.autoScroll {
				m.viewport.GotoBottom()
			}
		}
	}

	return m, tea.Batch(cmds...)
}

func (m AttachDashboardModel) View() string {
	header := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("205")).
		Render(fmt.Sprintf("Session: %s [%s]", m.sessionName, m.status))

	goalText := m.goal
	if len(goalText) > 60 {
		goalText = goalText[:57] + "..."
	}
	goal := lipgloss.NewStyle().
		Foreground(lipgloss.Color("240")).
		Render(fmt.Sprintf("Goal: %s", goalText))

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Render("q: quit • s: toggle auto-scroll")

	return fmt.Sprintf("%s\n%s\n%s\n%s", header, goal, m.viewport.View(), help)
}

func readLogsCmd(file *os.File) tea.Cmd {
	return func() tea.Msg {
		// Read new data from file
		content, err := io.ReadAll(file)
		if err != nil {
			return nil
		}
		if len(content) == 0 {
			return logUpdateMsg(nil)
		}

		entries, _ := ParseLogLines(content)
		return logUpdateMsg(entries)
	}
}

func renderEntries(entries []LogEntry) string {
	var sb strings.Builder
	for _, e := range entries {
		sb.WriteString(fmt.Sprintf("[%s] %s: %s\n", e.Time.Format("15:04:05"), e.Level, e.Msg))
	}
	return sb.String()
}

// StartAttachDashboard starts the TUI.
func StartAttachDashboard(sessionName string, sm runner.ISessionManager) error {
	model, err := NewAttachDashboardModel(sessionName, sm)
	if err != nil {
		return err
	}
	// Ensure file is closed when program exits
	defer model.file.Close()

	p := tea.NewProgram(model, tea.WithAltScreen())
	_, err = p.Run()
	return err
}
