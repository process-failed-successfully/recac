package ui

import (
	"fmt"
	"recac/internal/model"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Callbacks for actions
type ActionCallbacks struct {
	GetSessions func() ([]model.UnifiedSession, error)
	Stop        func(name string) error
	Pause       func(name string) error
	Resume      func(name string) error
	GetLogs     func(name string) (string, error)
}

type MonitorDashboardModel struct {
	table         table.Model
	viewport      viewport.Model
	callbacks     ActionCallbacks
	sessions      []model.UnifiedSession
	lastUpdate    time.Time
	err           error
	width         int
	height        int
	selectedRow   int
	viewMode      string // "list", "logs", "confirm_kill"
	logContent    string
	message       string // Status message (e.g., "Session stopped")
	sessionToKill string
}

type monitorTickMsg time.Time
type monitorSessionsRefreshedMsg []model.UnifiedSession
type actionResultMsg struct {
	err error
	msg string
}

var (
	monitorTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	monitorHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(1)
	messageStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("205")).Bold(true).MarginTop(1)
	confirmStyle      = lipgloss.NewStyle().Foreground(lipgloss.Color("230")).Background(lipgloss.Color("160")).Padding(1, 2).Bold(true).Border(lipgloss.DoubleBorder()).MarginTop(1)
)

func NewMonitorDashboardModel(callbacks ActionCallbacks) MonitorDashboardModel {
	columns := []table.Column{
		{Title: "NAME", Width: 20},
		{Title: "STATUS", Width: 10},
		{Title: "LOCATION", Width: 8},
		{Title: "COST", Width: 10},
		{Title: "GOAL", Width: 50},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(15),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	vp := viewport.New(0, 0)

	return MonitorDashboardModel{
		table:     t,
		viewport:  vp,
		callbacks: callbacks,
		viewMode:  "list",
	}
}

func (m MonitorDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		refreshMonitorSessionsCmd(m.callbacks.GetSessions),
		tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return monitorTickMsg(t)
		}),
	)
}

func (m MonitorDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(m.width)
		m.table.SetHeight(m.height - 10) // Reserve space for header/footer/help
		m.viewport.Width = m.width
		m.viewport.Height = m.height - 5

	case tea.KeyMsg:
		if m.viewMode == "logs" {
			switch msg.String() {
			case "q", "esc":
				m.viewMode = "list"
				return m, nil
			default:
				var vpCmd tea.Cmd
				m.viewport, vpCmd = m.viewport.Update(msg)
				return m, vpCmd
			}
		}

		if m.viewMode == "confirm_kill" {
			switch msg.String() {
			case "y", "Y":
				name := m.sessionToKill
				m.sessionToKill = ""
				m.viewMode = "list"
				return m, func() tea.Msg {
					err := m.callbacks.Stop(name)
					if err != nil {
						return actionResultMsg{err: err}
					}
					return actionResultMsg{msg: fmt.Sprintf("Stopped session %s", name)}
				}
			case "n", "N", "esc", "q":
				m.sessionToKill = ""
				m.viewMode = "list"
				return m, nil
			}
			return m, nil
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "k":
			if selected := m.table.SelectedRow(); selected != nil {
				m.sessionToKill = selected[0]
				m.viewMode = "confirm_kill"
				return m, nil
			}
		case "p":
			if selected := m.table.SelectedRow(); selected != nil {
				name := selected[0]
				status := selected[1]
				return m, func() tea.Msg {
					var err error
					action := ""
					if status == "paused" {
						err = m.callbacks.Resume(name)
						action = "Resumed"
					} else if status == "running" {
						err = m.callbacks.Pause(name)
						action = "Paused"
					} else {
						return actionResultMsg{err: fmt.Errorf("cannot toggle pause for status: %s", status)}
					}
					if err != nil {
						return actionResultMsg{err: err}
					}
					return actionResultMsg{msg: fmt.Sprintf("%s session %s", action, name)}
				}
			}
		case "l", "enter":
			if selected := m.table.SelectedRow(); selected != nil {
				name := selected[0]
				return m, func() tea.Msg {
					logs, err := m.callbacks.GetLogs(name)
					if err != nil {
						return actionResultMsg{err: err}
					}
					return func() tea.Msg { return logs }() // Hacky way to pass string, better distinct msg type
				}
			}
		}

	case monitorTickMsg:
		return m, tea.Batch(
			refreshMonitorSessionsCmd(m.callbacks.GetSessions),
			tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
				return monitorTickMsg(t)
			}),
		)

	case monitorSessionsRefreshedMsg:
		m.sessions = msg
		m.lastUpdate = time.Now()
		m.updateTableRows()

	case actionResultMsg:
		if msg.err != nil {
			m.message = fmt.Sprintf("Error: %v", msg.err)
		} else {
			m.message = msg.msg
		}
		// Clear message after 3 seconds
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return actionResultMsg{msg: ""}
		})

	case string: // Logs content (from 'l' key)
		m.logContent = msg
		m.viewMode = "logs"
		m.viewport.SetContent(m.logContent)
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *MonitorDashboardModel) updateTableRows() {
	rows := []table.Row{}
	for _, s := range m.sessions {
		goal := s.Goal
		if len(goal) > 50 {
			goal = goal[:47] + "..."
		}
		cost := "N/A"
		if s.HasCost {
			cost = fmt.Sprintf("$%.4f", s.Cost)
		}

		rows = append(rows, table.Row{
			s.Name,
			s.Status,
			s.Location,
			cost,
			goal,
		})
	}
	m.table.SetRows(rows)
	// Restore cursor if possible (basic implementation resets or keeps index if valid)
}

func (m MonitorDashboardModel) View() string {
	if m.viewMode == "logs" {
		return fmt.Sprintf("%s\n\n%s\n\n(Press q/esc to back)",
			monitorTitleStyle.Render("Session Logs"),
			m.viewport.View())
	}

	if m.viewMode == "confirm_kill" {
		return fmt.Sprintf("\n%s\n\nAre you sure you want to kill session '%s'?\n\n(y/n)",
			confirmStyle.Render("⚠️  DANGER ZONE"),
			m.sessionToKill)
	}

	s := monitorTitleStyle.Render("RECAC Control Center") + "\n"
	s += fmt.Sprintf("Last updated: %s\n\n", m.lastUpdate.Format("15:04:05"))

	if len(m.sessions) == 0 {
		s += "\n  No active sessions found.\n  Run 'recac start' to create a new session.\n\n"
	} else {
		s += m.table.View() + "\n"
	}

	if m.message != "" {
		s += messageStyle.Render(m.message) + "\n"
	}

	s += monitorHelpStyle.Render("Keys: ↑/↓ navigate • k: kill • p: pause/resume • l/enter: logs • q: quit")

	return s
}

func refreshMonitorSessionsCmd(getSessions func() ([]model.UnifiedSession, error)) tea.Cmd {
	return func() tea.Msg {
		if getSessions == nil {
			return nil
		}
		sessions, err := getSessions()
		if err != nil {
			return actionResultMsg{err: err}
		}
		return monitorSessionsRefreshedMsg(sessions)
	}
}

// StartMonitorDashboard starts the monitor TUI
func StartMonitorDashboard(callbacks ActionCallbacks) error {
	p := tea.NewProgram(NewMonitorDashboardModel(callbacks), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
