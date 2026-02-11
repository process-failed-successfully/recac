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

type psDashboardModel struct {
	table       table.Model
	viewport    viewport.Model
	sessions    []model.UnifiedSession
	lastUpdate  time.Time
	err         error
	width       int
	height      int
	showCosts   bool
	sortBy      string
	viewingLogs bool
}

type psTickMsg time.Time
type psSessionsRefreshedMsg []model.UnifiedSession

var psDashboardTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
var psLogTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))

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
	vp.Style = lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("62")).
		PaddingRight(2)

	return psDashboardModel{
		table:     t,
		viewport:  vp,
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
		m.viewport.Height = m.height - 8 // Adjust for header/footer
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "enter":
			if !m.viewingLogs {
				// Enter logs view
				selectedRow := m.table.SelectedRow()
				if selectedRow != nil && len(selectedRow) > 0 {
					sessionName := selectedRow[0]
					var selectedSession *model.UnifiedSession
					for i := range m.sessions {
						if m.sessions[i].Name == sessionName {
							selectedSession = &m.sessions[i]
							break
						}
					}
					if selectedSession != nil {
						logs := selectedSession.Logs
						if logs == "" {
							logs = "No logs available. Run with --logs flag to capture logs."
						}
						m.viewport.SetContent(logs)
						m.viewingLogs = true
						return m, nil
					}
				}
			} else {
				// Exit logs view
				m.viewingLogs = false
				return m, nil
			}
		case "esc":
			if m.viewingLogs {
				m.viewingLogs = false
				return m, nil
			}
		}

	case psTickMsg:
		if !m.viewingLogs {
			return m, refreshPsSessionsCmd()
		}
		// Continue ticking even if viewing logs, but don't refresh yet to avoid content jump
		// Or maybe we should refresh logs? For now, keep static while viewing.
		return m, tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
			return psTickMsg(t)
		})

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

	if m.viewingLogs {
		m.viewport, cmd = m.viewport.Update(msg)
	} else {
		m.table, cmd = m.table.Update(msg)
	}
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

	if m.viewingLogs {
		var s strings.Builder
		s.WriteString(psLogTitleStyle.Render(" LOGS VIEW (Press Esc or Enter to return)") + "\n")
		s.WriteString(m.viewport.View())
		return s.String()
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

var StartPsDashboard = func(showCosts bool, sortBy string) error {
	p := tea.NewProgram(NewPsDashboardModel(showCosts, sortBy), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
