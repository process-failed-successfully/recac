package ui

import (
	"fmt"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

import (
	"recac/internal/runner"
)

// sessionsRefreshedMsg is a message sent when the session list has been updated.
type sessionsRefreshedMsg struct {
	sessions []*runner.SessionState
	err      error
}

type DashboardModel struct {
	table          table.Model
	sessionManager ISessionManager
	loading        bool
	err            error
}

func NewDashboardModel(sm ISessionManager) DashboardModel {
	columns := []table.Column{
		{Title: "Session", Width: 20},
		{Title: "Status", Width: 10},
		{Title: "Start Time", Width: 20},
	}

	// Mock data is removed, will be loaded dynamically
	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
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

	return DashboardModel{
		table:          t,
		sessionManager: sm,
		loading:        true,
	}
}

type dashboardTickMsg time.Time

func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		refreshSessionsCmd(m.sessionManager),
		tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return dashboardTickMsg(t)
		}),
	)
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case dashboardTickMsg:
		m.loading = true
		return m, refreshSessionsCmd(m.sessionManager)
	case sessionsRefreshedMsg:
		m.loading = false
		if msg.err != nil {
			m.err = msg.err
			return m, nil
		}
		rows := make([]table.Row, len(msg.sessions))
		for i, session := range msg.sessions {
			rows[i] = table.Row{
				session.Name,
				string(session.Status),
				session.StartTime.Format(time.RFC1123),
			}
		}
		m.table.SetRows(rows)
		return m, tea.Tick(5*time.Second, func(t time.Time) tea.Msg {
			return dashboardTickMsg(t)
		})
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m DashboardModel) View() string {
	if m.loading {
		return "Loading sessions..."
	}
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Render("recac Dashboard"),
		m.table.View(),
	)
}

// StartDashboard launches the TUI dashboard.
func StartDashboard(sm ISessionManager) error {
	p := tea.NewProgram(NewDashboardModel(sm))
	if _, err := p.Run(); err != nil {
		return fmt.Errorf("error running dashboard: %w", err)
	}
	return nil
}

// refreshSessionsCmd is a command that fetches the latest session data.
func refreshSessionsCmd(sm ISessionManager) tea.Cmd {
	return func() tea.Msg {
		sessions, err := sm.ListSessions()
		if err != nil {
			return sessionsRefreshedMsg{err: err}
		}
		return sessionsRefreshedMsg{sessions: sessions}
	}
}
