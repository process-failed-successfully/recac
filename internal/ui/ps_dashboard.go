package ui

import (
	"fmt"
	"recac/internal/model"
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
}

type psTickMsg time.Time
type psSessionsRefreshedMsg []model.UnifiedSession

var psDashboardTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))

func NewPsDashboardModel() psDashboardModel {
	columns := []table.Column{
		{Title: "NAME", Width: 25},
		{Title: "STATUS", Width: 10},
		{Title: "LOCATION", Width: 10},
		{Title: "LAST USED", Width: 15},
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

	return psDashboardModel{table: t}
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
		lastUsed := formatSince(s.LastActivity)
		if s.Location == "k8s" {
			lastUsed = formatSince(s.StartTime)
		}
		goal := s.Goal
		if len(goal) > 57 {
			goal = goal[:57] + "..."
		}
		rows = append(rows, table.Row{
			s.Name,
			s.Status,
			s.Location,
			lastUsed,
			goal,
		})
	}
	m.table.SetRows(rows)
}

func (m psDashboardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var s strings.Builder
	s.WriteString(psDashboardTitleStyle.Render(" RECAC PS Dashboard") + "\n")
	s.WriteString(fmt.Sprintf("Last updated: %s (press 'q' to quit)\n\n", m.lastUpdate.Format(time.RFC1123)))

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

var StartPsDashboard = func() error {
	p := tea.NewProgram(NewPsDashboardModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}

// formatSince returns a human-readable string representing the time elapsed since t.
// This is duplicated from ps.go to avoid package cycle.
func formatSince(t time.Time) string {
	if t.IsZero() {
		return "never"
	}
	since := time.Since(t)
	if since < time.Minute {
		return fmt.Sprintf("%ds ago", int(since.Seconds()))
	}
	if since < time.Hour {
		return fmt.Sprintf("%dm ago", int(since.Minutes()))
	}
	if since < 24*time.Hour {
		return fmt.Sprintf("%dh ago", int(since.Hours()))
	}
	if since < 7*24*time.Hour {
		return fmt.Sprintf("%dd ago", int(since.Hours()/24))
	}
	return t.Format("2006-01-02")
}
