package ui

import (
	"fmt"
	"recac/internal/model"
	"recac/internal/utils"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// This func will be set by the caller in the cmd package
var GetSessions func() ([]model.UnifiedSession, error)

type psDashboardModel struct {
	table      table.Model
	sessions   []model.UnifiedSession
	lastUpdate time.Time
	err        error
	width      int
	height     int
	showCosts  bool
	sortBy     string

	// Detail view state
	selectedSession model.UnifiedSession
	showDetails     bool
}

type psTickMsg time.Time
type psSessionsRefreshedMsg []model.UnifiedSession

var psDashboardTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))

func NewPsDashboardModel(showCosts bool, sortBy string) psDashboardModel {
	columns := []table.Column{
		{Title: "NAME", Width: 25},
		{Title: "STATUS", Width: 10},
		{Title: "CPU", Width: 8},
		{Title: "MEM", Width: 8},
		{Title: "LOCATION", Width: 10},
		{Title: "LAST USED", Width: 15},
		{Title: "GOAL", Width: 60},
	}

	if showCosts {
		columns = append(columns, []table.Column{
			{Title: "PROMPT", Width: 8},
			{Title: "COMPL", Width: 8},
			{Title: "TOTAL", Width: 8},
			{Title: "COST", Width: 10},
		}...)
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

	return psDashboardModel{
		table:     t,
		showCosts: showCosts,
		sortBy:    sortBy,
	}
}

func (m psDashboardModel) Init() tea.Cmd {
	return tea.Batch(refreshPsSessionsCmd(), tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return psTickMsg(t)
	}))
}

func (m psDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
		case "enter":
			if !m.showDetails {
				idx := m.table.Cursor()
				if idx >= 0 && idx < len(m.sessions) {
					m.selectedSession = m.sessions[idx]
					m.showDetails = true
				}
			}
		case "esc":
			if m.showDetails {
				m.showDetails = false
				return m, nil
			}
		}

	case psTickMsg:
		return m, refreshPsSessionsCmd()

	case psSessionsRefreshedMsg:
		m.sessions = msg
		m.lastUpdate = time.Now()
		m.sortSessions()
		m.updateTableRows()
		return m, nil

	case error:
		m.err = msg
		return m, nil
	}

	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *psDashboardModel) sortSessions() {
	sort.SliceStable(m.sessions, func(i, j int) bool {
		switch m.sortBy {
		case "cost":
			if m.sessions[i].HasCost && m.sessions[j].HasCost {
				return m.sessions[i].Cost > m.sessions[j].Cost
			}
			return m.sessions[i].HasCost
		case "name":
			return m.sessions[i].Name < m.sessions[j].Name
		case "time":
			fallthrough
		default:
			return m.sessions[i].StartTime.After(m.sessions[j].StartTime)
		}
	})
}

func (m *psDashboardModel) updateTableRows() {
	rows := []table.Row{}
	for _, s := range m.sessions {
		lastUsed := utils.FormatSince(s.LastActivity)
		if s.Location == "k8s" {
			lastUsed = utils.FormatSince(s.StartTime)
		}
		goal := s.Goal
		if len(goal) > 54 {
			goal = goal[:54] + "..."
		}

		row := table.Row{
			s.Name,
			s.Status,
			s.CPU,
			s.Memory,
			s.Location,
			lastUsed,
			goal,
		}

		if m.showCosts {
			if s.HasCost {
				row = append(row,
					fmt.Sprintf("%d", s.Tokens.TotalPromptTokens),
					fmt.Sprintf("%d", s.Tokens.TotalResponseTokens),
					fmt.Sprintf("%d", s.Tokens.TotalTokens),
					fmt.Sprintf("$%.6f", s.Cost),
				)
			} else {
				row = append(row, "N/A", "N/A", "N/A", "N/A")
			}
		}

		rows = append(rows, row)
	}
	m.table.SetRows(rows)
}

func (m psDashboardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	if m.showDetails {
		return m.viewDetails()
	}

	var s strings.Builder
	s.WriteString(psDashboardTitleStyle.Render(" RECAC PS Dashboard") + "\n")
	s.WriteString(fmt.Sprintf("Last updated: %s (press 'q' to quit, 'enter' for details)\n\n", m.lastUpdate.Format(time.RFC1123)))

	s.WriteString(m.table.View())
	return s.String()
}

func (m psDashboardModel) viewDetails() string {
	s := strings.Builder{}
	session := m.selectedSession

	// Styles
	labelStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Bold(true)
	valueStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	titleStyle := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62")).MarginBottom(1)

	s.WriteString(titleStyle.Render(fmt.Sprintf("Session Details: %s", session.Name)) + "\n")

	fields := []struct {
		Label string
		Value string
	}{
		{"Status", session.Status},
		{"Location", session.Location},
		{"Start Time", session.StartTime.Format(time.RFC1123)},
		{"Last Activity", session.LastActivity.Format(time.RFC1123)},
		{"CPU", session.CPU},
		{"Memory", session.Memory},
	}

	if session.HasCost {
		fields = append(fields, struct{ Label, Value string }{"Cost", fmt.Sprintf("$%.6f", session.Cost)})
		fields = append(fields, struct{ Label, Value string }{"Total Tokens", fmt.Sprintf("%d", session.Tokens.TotalTokens)})
	}

	for _, f := range fields {
		s.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render(f.Label+":"), valueStyle.Render(f.Value)))
	}

	s.WriteString(fmt.Sprintf("\n%s\n%s\n", labelStyle.Render("Goal:"), valueStyle.Render(session.Goal)))

	if session.Logs != "" {
		s.WriteString(fmt.Sprintf("\n%s\n", labelStyle.Render("Logs (Preview):")))
		lines := strings.Split(session.Logs, "\n")
		limit := 10
		if len(lines) > limit {
			lines = lines[:limit]
		}
		for _, line := range lines {
			if line != "" {
				s.WriteString(fmt.Sprintf("  %s\n", line))
			}
		}
		if len(strings.Split(session.Logs, "\n")) > limit {
			s.WriteString("  ...\n")
		}
	}

	s.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Render("(press 'esc' to back)"))
	return s.String()
}

func refreshPsSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		if GetSessions == nil {
			return fmt.Errorf("GetSessions function is not set")
		}
		sessions, err := GetSessions()
		if err != nil {
			return err
		}
		return psSessionsRefreshedMsg(sessions)
	}
}

var StartPsDashboard = func(showCosts bool, sortBy string) error {
	p := tea.NewProgram(NewPsDashboardModel(showCosts, sortBy), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
