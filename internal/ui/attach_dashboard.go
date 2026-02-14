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
	attachTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).MarginLeft(2)
	attachInfoStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).MarginLeft(2)
)

type AttachDashboardModel struct {
	sessionName string
	logFile     string
	viewport    viewport.Model
	err         error
	ready       bool
}

type logTickMsg time.Time

func NewAttachDashboardModel(sessionName, logFile string) AttachDashboardModel {
	return AttachDashboardModel{
		sessionName: sessionName,
		logFile:     logFile,
	}
}

func (m AttachDashboardModel) Init() tea.Cmd {
	return tea.Tick(time.Second/2, func(t time.Time) tea.Msg {
		return logTickMsg(t)
	})
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
			m.viewport.HighPerformanceRendering = false
			m.ready = true

			// Initial load
			content, err := os.ReadFile(m.logFile)
			if err != nil {
				m.err = err
				m.viewport.SetContent(fmt.Sprintf("Error reading log file: %v", err))
			} else {
				m.viewport.SetContent(string(content))
				m.viewport.GotoBottom()
			}
		} else {
			m.viewport.Width = msg.Width
			m.viewport.Height = msg.Height - verticalMarginHeight
		}

	case logTickMsg:
		// Re-read logs
		if m.ready {
			content, err := os.ReadFile(m.logFile)
			if err != nil {
				m.err = err
			} else {
				newContent := string(content)
				atBottom := m.viewport.AtBottom()
				m.viewport.SetContent(newContent)
				if atBottom {
					m.viewport.GotoBottom()
				}
			}
		}

		cmds = append(cmds, tea.Tick(time.Second/2, func(t time.Time) tea.Msg {
			return logTickMsg(t)
		}))
	}

	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m AttachDashboardModel) View() string {
	if !m.ready {
		return "\n  Initializing..."
	}
	if m.err != nil {
		return fmt.Sprintf("\n  Error: %v\n\n  Press q to quit.", m.err)
	}
	return fmt.Sprintf("%s\n%s\n%s", m.headerView(), m.viewport.View(), m.footerView())
}

func (m AttachDashboardModel) headerView() string {
	title := attachTitleStyle.Render(fmt.Sprintf("Session: %s", m.sessionName))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(title)))
	return lipgloss.JoinHorizontal(lipgloss.Center, title, line)
}

func (m AttachDashboardModel) footerView() string {
	info := attachInfoStyle.Render(fmt.Sprintf("%s • Press q to detach", m.logFile))
	line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(info)))
	return lipgloss.JoinHorizontal(lipgloss.Center, line, info)
}

func StartAttachDashboard(sessionName, logFile string) error {
	p := tea.NewProgram(NewAttachDashboardModel(sessionName, logFile), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
