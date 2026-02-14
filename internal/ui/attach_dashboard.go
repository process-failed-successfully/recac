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
	attachTitleStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FAFAFA")).
				Background(lipgloss.Color("#7D56F4")).
				Padding(0, 1)

	attachInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("241"))
)

type attachTickMsg time.Time

type attachDashboardModel struct {
	viewport   viewport.Model
	sessionName string
	logPath    string
	content    string
	fileOffset int64
	err        error
	ready      bool
}

func NewAttachDashboardModel(sessionName, logPath string) attachDashboardModel {
	return attachDashboardModel{
		sessionName: sessionName,
		logPath:    logPath,
		fileOffset: 0,
	}
}

func (m attachDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}),
	)
}

func (m attachDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		verticalMargin := headerHeight + footerHeight

		if !m.ready {
			m.viewport = viewport.New(msg.Width, msg.Height-verticalMargin)
			m.viewport.YPosition = headerHeight
			m.viewport.HighPerformanceRendering = false // simpler for now
			m.ready = true

			// Initial load
			m.readLogUpdates()
			m.viewport.SetContent(m.content)
			m.viewport.GotoBottom()
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMargin
		}

	case attachTickMsg:
		updated := m.readLogUpdates()
		if updated {
			m.viewport.SetContent(m.content)
			m.viewport.GotoBottom()
		}
		cmds = append(cmds, tea.Tick(500*time.Millisecond, func(t time.Time) tea.Msg {
			return attachTickMsg(t)
		}))
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m *attachDashboardModel) readLogUpdates() bool {
	file, err := os.Open(m.logPath)
	if err != nil {
		m.err = err
		return false
	}
	defer file.Close()

	stat, err := file.Stat()
	if err != nil {
		m.err = err
		return false
	}

	if stat.Size() <= m.fileOffset {
		return false // No new content
	}

	// Seek to last offset
	_, err = file.Seek(m.fileOffset, 0)
	if err != nil {
		m.err = err
		return false
	}

	// Read new content
	newBytes := make([]byte, stat.Size()-m.fileOffset)
	n, err := file.Read(newBytes)
	if err != nil && n == 0 {
		return false
	}

	m.content += string(newBytes[:n])
	m.fileOffset += int64(n)
	return true
}

func (m attachDashboardModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m attachDashboardModel) headerView() string {
	title := attachTitleStyle.Render(fmt.Sprintf("Session: %s", m.sessionName))
	line := strings.Repeat("─", attachMax(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m attachDashboardModel) footerView() string {
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
func StartAttachDashboard(sessionName, logPath string) error {
	// Initial read to populate content before first render if possible
	// But actually Update handles initial load on WindowSizeMsg which happens early.

	p := tea.NewProgram(NewAttachDashboardModel(sessionName, logPath), tea.WithAltScreen(), tea.WithMouseCellMotion())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
