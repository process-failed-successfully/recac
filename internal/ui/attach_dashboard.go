package ui

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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
	viewport   viewport.Model
	logFile    string
	ready      bool
	header     string
	footer     string
	content    string
	offset     int64
	err        error
}

type readLogsMsg struct {
	content string
	offset  int64
	err     error
}

type attachTickMsg time.Time

func NewAttachDashboardModel(logFile string) AttachDashboardModel {
	return AttachDashboardModel{
		logFile: logFile,
		header:  "Session Logs",
		footer:  "Press q or esc to quit",
	}
}

func readLogsCmd(filename string, offset int64) tea.Cmd {
	return func() tea.Msg {
		f, err := os.Open(filename)
		if err != nil {
			// If file doesn't exist yet, just wait
			if os.IsNotExist(err) {
				return readLogsMsg{content: "", offset: offset}
			}
			return readLogsMsg{err: err}
		}
		defer f.Close()

		stat, err := f.Stat()
		if err != nil {
			return readLogsMsg{err: err}
		}

		if stat.Size() <= offset {
			return readLogsMsg{content: "", offset: offset}
		}

		readSize := stat.Size() - offset
		buf := make([]byte, readSize)
		_, err = f.ReadAt(buf, offset)
		if err != nil {
			return readLogsMsg{err: err}
		}

		return readLogsMsg{content: string(buf), offset: stat.Size()}
	}
}

func (m AttachDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}),
		readLogsCmd(m.logFile, 0),
	)
}

func (m AttachDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if k := msg.String(); k == "ctrl+c" || k == "q" || k == "esc" {
			return m, tea.Quit
		}

	case tea.WindowSizeMsg:
		headerHeight := lipgloss.Height(m.headerView())
		footerHeight := lipgloss.Height(m.footerView())
		verticalMarginHeight := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMarginHeight)
			m.viewport.YPosition = headerHeight
			m.viewport.SetContent(m.content)
			m.ready = true
			m.viewport.GotoBottom()
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

	case attachTickMsg:
		cmds = append(cmds, readLogsCmd(m.logFile, m.offset))
		cmds = append(cmds, tea.Tick(250*time.Millisecond, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}))

	case readLogsMsg:
		if msg.err != nil {
			m.err = msg.err
		} else if msg.content != "" {
			m.content += msg.content
			m.offset = msg.offset

			atBottom := m.viewport.AtBottom()
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
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.", m.err)
	}
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m AttachDashboardModel) headerView() string {
	title := attachTitleStyle.Render(m.header)
	line := strings.Repeat("─", attachMax(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m AttachDashboardModel) footerView() string {
	info := attachInfoStyle.Render(fmt.Sprintf("%3.f%%", m.viewport.ScrollPercent()*100))
	line := strings.Repeat("─", attachMax(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func attachMax(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// StartAttachDashboard starts the attach TUI
func StartAttachDashboard(logFile string) error {
	m := NewAttachDashboardModel(logFile)
	p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
