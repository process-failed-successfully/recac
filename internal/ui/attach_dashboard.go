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

// This func will be set by the caller in the cmd package
var GetSession func(string) (*runner.SessionState, error)

type attachTickMsg time.Time

type attachDashboardModel struct {
	sessionName string
	viewport    viewport.Model
	logFile     *os.File
	lastOffset  int64
	logContent  string
	session     *runner.SessionState
	err         error
	width       int
	height      int
	ready       bool
}

var (
	attachTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	infoStyle        = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	statusRunning    = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	statusCompleted  = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	statusError      = lipgloss.NewStyle().Foreground(lipgloss.Color("160"))
)

const maxLogSize = 1 * 1024 * 1024 // 1MB

func NewAttachDashboardModel(sessionName string) attachDashboardModel {
	return attachDashboardModel{
		sessionName: sessionName,
	}
}

func (m attachDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}),
		readLogsCmd(m.sessionName, 0), // Initial read
	)
}

func (m attachDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c", "esc":
			if m.logFile != nil {
				m.logFile.Close()
			}
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		headerHeight := 4
		footerHeight := 1
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
		cmds := []tea.Cmd{
			tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
				return attachTickMsg(t)
			}),
		}

		// Refresh session state
		if GetSession != nil {
			s, err := GetSession(m.sessionName)
			if err == nil {
				m.session = s
			}
		}

		// Read new logs
		if m.session != nil {
			cmds = append(cmds, readLogsCmd(m.sessionName, m.lastOffset))
		}

		return m, tea.Batch(cmds...)

	case logReadMsg:
		if msg.err != nil {
			// Ignore errors for now or log them properly?
			// m.err = msg.err
		} else {
			// If newOffset is less than lastOffset, it means the file was truncated or rotated
			if msg.newOffset < m.lastOffset {
				m.logContent = msg.content
				m.viewport.SetContent(m.logContent)
				m.viewport.GotoBottom()
				m.lastOffset = msg.newOffset
			} else if msg.content != "" {
				m.logContent += msg.content

				// Enforce max log size
				if len(m.logContent) > maxLogSize {
					cut := len(m.logContent) - maxLogSize
					// Find next newline to keep it clean
					if idx := strings.IndexByte(m.logContent[cut:], '\n'); idx != -1 {
						cut += idx + 1
					}
					m.logContent = m.logContent[cut:]
				}

				m.viewport.SetContent(m.logContent)
				m.viewport.GotoBottom()
				m.lastOffset = msg.newOffset
			}
		}
	}

	m.viewport, cmd = m.viewport.Update(msg)
	return m, cmd
}

type logReadMsg struct {
	content   string
	newOffset int64
	err       error
}

func readLogsCmd(sessionName string, offset int64) tea.Cmd {
	return func() tea.Msg {
		if GetSession == nil {
			return logReadMsg{err: fmt.Errorf("GetSession not set")}
		}
		session, err := GetSession(sessionName)
		if err != nil {
			return logReadMsg{err: err}
		}

		f, err := os.Open(session.LogFile)
		if err != nil {
			return logReadMsg{err: err}
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			return logReadMsg{err: err}
		}

		if stat.Size() < offset {
			// Log file truncated or rotated
			offset = 0
		}

		if stat.Size() == offset {
			return logReadMsg{newOffset: offset}
		}

		_, err = f.Seek(offset, 0)
		if err != nil {
			return logReadMsg{err: err}
		}

		content, err := io.ReadAll(f)
		if err != nil {
			return logReadMsg{err: err}
		}

		return logReadMsg{
			content:   string(content),
			newOffset: offset + int64(len(content)),
		}
	}
}

func (m attachDashboardModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}

	header := m.headerView()
	footer := m.footerView()

	return fmt.Sprintf("%s\n%s\n%s", header, m.viewport.View(), footer)
}

func (m attachDashboardModel) headerView() string {
	title := attachTitleStyle.Render(" RECAC ATTACH ")
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))

	status := "Unknown"
	pid := 0
	goal := "N/A"

	if m.session != nil {
		status = m.session.Status
		pid = m.session.PID
		goal = m.session.Goal
	}

	statusStyle := infoStyle
	switch strings.ToLower(status) {
	case "running":
		statusStyle = statusRunning
	case "completed":
		statusStyle = statusCompleted
	case "error":
		statusStyle = statusError
	}

	info := fmt.Sprintf("Session: %s | PID: %d | Status: %s", m.sessionName, pid, statusStyle.Render(status))
	goalStr := fmt.Sprintf("Goal: %s", goal)
	if len(goalStr) > m.viewport.Width {
		goalStr = goalStr[:m.viewport.Width-3] + "..."
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinHorizontal(lipgloss.Center, title, line),
		info,
		goalStr,
		strings.Repeat("─", m.viewport.Width),
	)
}

func (m attachDashboardModel) footerView() string {
	return infoStyle.Render("Press q to quit | arrows/pgup/pgdn to scroll")
}

func StartAttachDashboard(sessionName string) error {
	p := tea.NewProgram(NewAttachDashboardModel(sessionName), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
