package ui

import (
	"fmt"
	"os"
	"time"

	"recac/internal/runner"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type attachTickMsg time.Time

type logReadMsg struct {
	content   string
	newOffset int64
	err       error
}

type AttachDashboardModel struct {
	viewport     viewport.Model
	sessionName  string
	logPath      string
	fetchSession func() (*runner.SessionState, error)
	session      *runner.SessionState
	err          error
	content      string
	offset       int64
	autoScroll   bool
	ready        bool
	width        int
	height       int
	readingLogs  bool
}

var (
	attachTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#7D56F4")).
				Padding(0, 1)

	attachStatusStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#205")).
				MarginLeft(1)

	attachFooterStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#626262")).
				MarginTop(1)
)

func NewAttachDashboard(sessionName, logPath string, fetchSession func() (*runner.SessionState, error)) AttachDashboardModel {
	return AttachDashboardModel{
		sessionName:  sessionName,
		logPath:      logPath,
		fetchSession: fetchSession,
		autoScroll:   true,
	}
}

func (m AttachDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}),
		func() tea.Msg {
			// Initial session fetch
			s, err := m.fetchSession()
			if err != nil {
				return err
			}
			return s
		},
	)
}

func readLogsCmd(path string, offset int64) tea.Cmd {
	return func() tea.Msg {
		file, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				return logReadMsg{content: "", newOffset: offset, err: nil}
			}
			return logReadMsg{err: err}
		}
		defer file.Close()

		stat, err := file.Stat()
		if err != nil {
			return logReadMsg{err: err}
		}

		readOffset := offset
		if stat.Size() < offset {
			readOffset = 0
		}

		if stat.Size() == readOffset {
			return logReadMsg{content: "", newOffset: readOffset, err: nil}
		}

		_, err = file.Seek(readOffset, 0)
		if err != nil {
			return logReadMsg{err: err}
		}

		// Read in chunks (max 64KB)
		bufSize := stat.Size() - readOffset
		if bufSize > 64*1024 {
			bufSize = 64 * 1024
		}

		buf := make([]byte, bufSize)
		n, err := file.Read(buf)
		if err != nil {
			return logReadMsg{err: err}
		}

		// If truncated, we signal it by returning a smaller offset than input (implicitly handled by logic)
		// Actually if truncated, we might want to reload whole file or just new part?
		// If truncated, readOffset became 0. So we return content from start.

		// Wait, if truncated, we should probably tell the model to reset content?
		// If readOffset < offset, it implies reset.

		return logReadMsg{
			content:   string(buf[:n]),
			newOffset: readOffset + int64(n),
			err:       nil,
		}
	}
}

func (m AttachDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			return m, tea.Quit
		case " ":
			m.autoScroll = !m.autoScroll
		case "up", "k":
			m.autoScroll = false // User wants to scroll manually
		case "down", "j":
			if m.viewport.AtBottom() {
				m.autoScroll = true
			}
		}

	case tea.WindowSizeMsg:
		headerHeight := 2
		footerHeight := 2
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.ready = true
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}
		m.width = msg.Width
		m.height = msg.Height

	case attachTickMsg:
		// 1. Schedule Log Read if not busy
		if !m.readingLogs {
			m.readingLogs = true
			cmds = append(cmds, readLogsCmd(m.logPath, m.offset))
		}

		// 2. Schedule next tick
		cmds = append(cmds, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}))

		// 3. Trigger session update occasionally
		cmds = append(cmds, func() tea.Msg {
			s, err := m.fetchSession()
			if err != nil {
				return err
			}
			return s
		})

	case logReadMsg:
		m.readingLogs = false
		if msg.err != nil {
			m.err = msg.err
		} else {
			// Check for truncation (offset reset)
			if msg.newOffset < m.offset {
				m.content = msg.content // Replace content
			} else {
				m.content += msg.content // Append content
			}
			m.offset = msg.newOffset

			if msg.content != "" || msg.newOffset != m.offset {
				m.viewport.SetContent(m.content)
				if m.autoScroll {
					m.viewport.GotoBottom()
				}
			}
		}

	case *runner.SessionState:
		m.session = msg

	case error:
		m.err = msg
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m AttachDashboardModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	header := m.headerView()
	footer := m.footerView()

	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

func (m AttachDashboardModel) headerView() string {
	title := attachTitleStyle.Render(fmt.Sprintf(" Session: %s ", m.sessionName))

	status := "Unknown"
	pid := 0
	goal := ""

	if m.session != nil {
		status = m.session.Status
		pid = m.session.PID
		goal = m.session.Goal
		if len(goal) > 50 {
			goal = goal[:47] + "..."
		}
	}

	statusLine := fmt.Sprintf("Status: %s | PID: %d | Goal: %s", status, pid, goal)
	return fmt.Sprintf("%s%s", title, attachStatusStyle.Render(statusLine))
}

func (m AttachDashboardModel) footerView() string {
	scrollStatus := "Auto-scroll: ON"
	if !m.autoScroll {
		scrollStatus = "Auto-scroll: OFF"
	}

	help := fmt.Sprintf("%s | q: quit | space: toggle scroll | ↑/↓: scroll", scrollStatus)
	if m.err != nil {
		help += fmt.Sprintf(" | Error: %v", m.err)
	}

	return attachFooterStyle.Render(help)
}

// StartAttachDashboard initializes and runs the dashboard
func StartAttachDashboard(sessionName, logPath string, fetchSession func() (*runner.SessionState, error)) error {
	p := tea.NewProgram(
		NewAttachDashboard(sessionName, logPath, fetchSession),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
