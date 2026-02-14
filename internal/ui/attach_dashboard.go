package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"recac/internal/runner"
)

var (
	attachTitleStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Right = "├"
		return lipgloss.NewStyle().BorderStyle(b).Padding(0, 1)
	}()

	attachInfoStyle = func() lipgloss.Style {
		b := lipgloss.RoundedBorder()
		b.Left = "┤"
		return attachTitleStyle.BorderStyle(b)
	}()
)

type AttachDashboardModel struct {
	sessionName   string
	logFile       string
	viewport      viewport.Model
	ready         bool
	offset        int64
	content       string
	width         int
	height        int
	err           error
	sessionStatus string
	sessionPID    int
}

type logReadMsg struct {
	content string
	offset  int64
	err     error
}

type attachTickMsg time.Time

func NewAttachDashboardModel(sessionName, logFile string, status string, pid int) AttachDashboardModel {
	return AttachDashboardModel{
		sessionName:   sessionName,
		logFile:       logFile,
		sessionStatus: status,
		sessionPID:    pid,
	}
}

func (m AttachDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		readLogsCmd(m.logFile, m.offset),
		tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}),
	)
}

func (m AttachDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.HighPerformanceRendering = false // simpler for now
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		}

	case attachTickMsg:
		return m, tea.Batch(
			readLogsCmd(m.logFile, m.offset),
			tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return attachTickMsg(t)
			}),
		)

	case logReadMsg:
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}

		if msg.content != "" {
			atBottom := m.viewport.AtBottom()
			m.content += msg.content
			m.offset = msg.offset
			m.viewport.SetContent(m.content)

			if atBottom {
				m.viewport.GotoBottom()
			}
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m AttachDashboardModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m AttachDashboardModel) headerView() string {
	title := attachTitleStyle.Render(fmt.Sprintf("Session: %s", m.sessionName))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m AttachDashboardModel) footerView() string {
	info := attachInfoStyle.Render(fmt.Sprintf("PID: %d • Status: %s • %3.f%%", m.sessionPID, m.sessionStatus, m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func readLogsCmd(filename string, offset int64) tea.Cmd {
	return func() tea.Msg {
		file, err := os.Open(filename)
		if err != nil {
			return logReadMsg{err: err}
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			return logReadMsg{err: err}
		}

		if stat.Size() <= offset {
			return logReadMsg{offset: offset} // No new content
		}

		// Seek to last offset
		_, err = file.Seek(offset, 0)
		if err != nil {
			return logReadMsg{err: err}
		}

		// Read new content
		content, err := io.ReadAll(file)
		if err != nil {
			return logReadMsg{err: err}
		}

		return logReadMsg{
			content: string(content),
			offset:  offset + int64(len(content)),
		}
	}
}

// StartAttachDashboard starts the attach TUI
func StartAttachDashboard(sessionName string) error {
	sm, err := runner.NewSessionManager()
	if err != nil {
		return fmt.Errorf("failed to create session manager: %w", err)
	}

	session, err := sm.LoadSession(sessionName)
	if err != nil {
		return fmt.Errorf("session not found: %w", err)
	}

	logFile, err := sm.GetSessionLogs(sessionName)
	if err != nil {
		return fmt.Errorf("failed to get log file: %w", err)
	}

	m := NewAttachDashboardModel(sessionName, logFile, session.Status, session.PID)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("failed to run dashboard: %w", err)
	}

	return nil
}
