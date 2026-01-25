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
var GetSessions func() ([]model.UnifiedSession, error)

type psDashboardModel struct {
	table      table.Model
	sessions   []model.UnifiedSession
	lastUpdate time.Time
	err        error
	width      int
	height     int
	showCosts  bool
}

type psTickMsg time.Time
type psSessionsRefreshedMsg []model.UnifiedSession

var psDashboardTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))

func NewPsDashboardModel(showCosts bool) psDashboardModel {
	m := psDashboardModel{showCosts: showCosts}
	columns := m.getColumns(showCosts)

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

	m.table = t
	return m
}

func (m psDashboardModel) getColumns(showCosts bool) []table.Column {
	if showCosts {
		return []table.Column{
			{Title: "NAME", Width: 25},
			{Title: "STATUS", Width: 10},
			{Title: "COST", Width: 12},
			{Title: "TOKENS", Width: 10},
			{Title: "LOCATION", Width: 10},
			{Title: "LAST USED", Width: 15},
			{Title: "GOAL", Width: 60},
		}
	}
	return []table.Column{
		{Title: "NAME", Width: 25},
		{Title: "STATUS", Width: 10},
		{Title: "CPU", Width: 8},
		{Title: "MEM", Width: 8},
		{Title: "LOCATION", Width: 10},
		{Title: "LAST USED", Width: 15},
		{Title: "GOAL", Width: 60},
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
		case "c":
			m.showCosts = !m.showCosts
			m.table.SetColumns(m.getColumns(m.showCosts))
			m.updateTableRows()
			return m, nil
		}

	case psTickMsg:
		return m, refreshPsSessionsCmd()

	case psSessionsRefreshedMsg:
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

		if m.showCosts {
			cost := "N/A"
			tokens := "N/A"
			if s.HasCost {
				cost = fmt.Sprintf("$%.6f", s.Cost)
				tokens = fmt.Sprintf("%d", s.Tokens.TotalTokens)
			}
			rows = append(rows, table.Row{
				s.Name,
				s.Status,
				cost,
				tokens,
				s.Location,
				lastUsed,
				goal,
			})
		} else {
			rows = append(rows, table.Row{
				s.Name,
				s.Status,
				s.CPU,
				s.Memory,
				s.Location,
				lastUsed,
				goal,
			})
		}
	}
	m.table.SetRows(rows)
}

func (m psDashboardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var s strings.Builder
	s.WriteString(psDashboardTitleStyle.Render(" RECAC PS Dashboard") + "\n")
	s.WriteString(fmt.Sprintf("Last updated: %s (press 'c' to toggle costs, 'q' to quit)\n\n", m.lastUpdate.Format(time.RFC1123)))

	s.WriteString(m.table.View())
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

var StartPsDashboard = func(showCosts bool) error {
	p := tea.NewProgram(NewPsDashboardModel(showCosts), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
