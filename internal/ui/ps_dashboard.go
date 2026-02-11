package ui

import (
	"fmt"
	"recac/internal/model"
	"recac/internal/utils"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// This func will be set by the caller in the cmd package
var GetSessions func() ([]model.UnifiedSession, error)
var GetSessionLogs func(string) (string, error)

type psDashboardModel struct {
	table          table.Model
	viewport       viewport.Model
	showLogs       bool
	sessions       []model.UnifiedSession
	currentSession string
	lastUpdate     time.Time
	err            error
	width          int
	height         int
	showCosts      bool
	sortBy         string
}

type psTickMsg time.Time
type psSessionsRefreshedMsg []model.UnifiedSession

type sessionLogsMsg struct {
	Content string
	Error   error
}

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

	vp := viewport.New(0, 0)

	return psDashboardModel{
		table:     t,
		viewport:  vp,
		showLogs:  false,
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
		m.viewport.Width = m.width
		m.viewport.Height = m.height - 4 // Title + Header
		return m, nil

	case tea.KeyMsg:
		if m.showLogs {
			switch msg.String() {
			case "esc", "q", "backspace":
				m.showLogs = false
				m.currentSession = ""
				return m, nil
			default:
				m.viewport, cmd = m.viewport.Update(msg)
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			selectedRow := m.table.SelectedRow()
			if len(selectedRow) > 0 {
				sessionName := selectedRow[0]
				m.currentSession = sessionName
				return m, fetchSessionLogsCmd(sessionName)
			}
		}

	case psTickMsg:
		if !m.showLogs {
			return m, refreshPsSessionsCmd()
		}
		// Keep refreshing in background
		return m, tea.Batch(refreshPsSessionsCmd(), tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return psTickMsg(t)
		}))

	case psSessionsRefreshedMsg:
		m.sessions = msg
		m.lastUpdate = time.Now()
		m.sortSessions()
		m.updateTableRows()
		return m, nil

	case sessionLogsMsg:
		if msg.Error != nil {
			m.err = msg.Error
		} else {
			m.viewport.SetContent(msg.Content)
			m.viewport.GotoBottom() // Logs usually read from bottom
			m.showLogs = true
		}
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

	if m.showLogs {
		header := psDashboardTitleStyle.Render(fmt.Sprintf(" Logs: %s (press 'esc' to back)", m.currentSession))
		line := strings.Repeat("─", max(0, m.viewport.Width-lipgloss.Width(header)))
		return fmt.Sprintf("%s\n%s\n%s", header, line, m.viewport.View())
	}

	var s strings.Builder
	s.WriteString(psDashboardTitleStyle.Render(" RECAC PS Dashboard") + "\n")
	s.WriteString(fmt.Sprintf("Last updated: %s (press 'q' to quit, 'enter' to view logs)\n\n", m.lastUpdate.Format(time.RFC1123)))

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

func fetchSessionLogsCmd(name string) tea.Cmd {
	return func() tea.Msg {
		if GetSessionLogs == nil {
			return sessionLogsMsg{Error: fmt.Errorf("GetSessionLogs function is not set")}
		}
		logs, err := GetSessionLogs(name)
		return sessionLogsMsg{Content: logs, Error: err}
	}
}

var StartPsDashboard = func(showCosts bool, sortBy string) error {
	p := tea.NewProgram(NewPsDashboardModel(showCosts, sortBy), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
