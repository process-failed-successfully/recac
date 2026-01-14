package ui

import (
	"fmt"
	"recac/internal/model"
	"recac/internal/utils"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// This func will be set by the caller in the cmd package
var GetTopSessions func() ([]model.UnifiedSession, error)

type topDashboardModel struct {
	table      table.Model
	sessions   []model.UnifiedSession
	lastUpdate time.Time
	err        error
	width      int
	height     int
}

type topTickMsg time.Time
type topSessionsRefreshedMsg []model.UnifiedSession

var topDashboardTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))

func NewTopDashboardModel() topDashboardModel {
	columns := []table.Column{
		{Title: "NAME", Width: 25},
		{Title: "STATUS", Width: 10},
		{Title: "CPU", Width: 10},
		{Title: "MEM", Width: 10},
		{Title: "UPTIME", Width: 15},
		{Title: "GOAL", Width: 60},
	}

	t := table.New(
		table.WithColumns(columns),
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

	return topDashboardModel{table: t}
}

func (m topDashboardModel) Init() tea.Cmd {
	return tea.Batch(refreshTopSessionsCmd(), tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
		return topTickMsg(t)
	}))
}

func (m topDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.table.SetWidth(m.width)
		m.table.SetHeight(m.height - 8) // Adjust for header/footer
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case topTickMsg:
		return m, refreshTopSessionsCmd()

	case topSessionsRefreshedMsg:
		m.sessions = msg
		m.lastUpdate = time.Now()
		m.updateTableRows()
		return m, nil

	case error:
		m.err = msg
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *topDashboardModel) updateTableRows() {
	rows := []table.Row{}
	for _, s := range m.sessions {
		uptime := utils.FormatSince(s.StartTime)
		goal := s.Goal
		if len(goal) > 57 {
			goal = goal[:57] + "..."
		}
		rows = append(rows, table.Row{
			s.Name,
			s.Status,
			s.CPU,
			s.Memory,
			uptime,
			goal,
		})
	}
	m.table.SetRows(rows)
}

func (m topDashboardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var s strings.Builder
	s.WriteString(topDashboardTitleStyle.Render(" RECAC Top Dashboard") + "\n")
	s.WriteString(fmt.Sprintf("Last updated: %s (press 'q' to quit)\n\n", m.lastUpdate.Format(time.RFC1123)))

	s.WriteString(m.table.View())
	return s.String()
}

func refreshTopSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		if GetTopSessions == nil {
			return fmt.Errorf("GetTopSessions function is not set")
		}
		sessions, err := GetTopSessions()
		if err != nil {
			return err
		}
		return topSessionsRefreshedMsg(sessions)
	}
}

var StartTopDashboard = func() error {
	p := tea.NewProgram(NewTopDashboardModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
