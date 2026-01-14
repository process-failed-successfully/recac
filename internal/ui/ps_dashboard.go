package ui

import (
	"fmt"
	"recac/internal/model"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// This func will be set by the caller in the cmd package
var GetSessions func() ([]model.UnifiedSession, error)

// ISessionManagerDashboard defines the interface for session management actions
// needed by the dashboard. This avoids a direct dependency on the runner package.
type ISessionManagerDashboard interface {
	StopSession(name string) error
	ArchiveSession(name string) error
	GetSessionLogs(name string) (string, error)
	GetSessionDiff(name string) (string, error)
}

type viewState int

const (
	listView viewState = iota
	detailView
)

type psDashboardModel struct {
	state           viewState
	sessionManager  ISessionManagerDashboard
	table           table.Model
	sessions        []model.UnifiedSession
	selectedSession *model.UnifiedSession
	notification    string
	detailContent   string
	lastUpdate      time.Time
	err             error
	width           int
	height          int
	help            help.Model
	keys            psKeyMap
}

type psTickMsg time.Time
type psSessionsRefreshedMsg []model.UnifiedSession
type notificationMsg string
type detailContentMsg string

var psDashboardTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
var psDetailStyle = lipgloss.NewStyle().PaddingLeft(2)

func NewPsDashboardModel(sm ISessionManagerDashboard) psDashboardModel {
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
		table.WithFocused(true),
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

	m := psDashboardModel{
		state:          listView,
		sessionManager: sm,
		table:          t,
		keys:           psKeys,
		help:           help.New(),
	}
	m.updateKeyBindings()
	return m
}

func (m *psDashboardModel) updateKeyBindings() {
	isListView := m.state == listView
	m.keys.Enter.SetEnabled(isListView)
	m.keys.Up.SetEnabled(isListView)
	m.keys.Down.SetEnabled(isListView)

	isDetailView := m.state == detailView
	m.keys.Back.SetEnabled(isDetailView)
	m.keys.Log.SetEnabled(isDetailView)
	m.keys.Diff.SetEnabled(isDetailView)
	m.keys.Stop.SetEnabled(isDetailView)
	m.keys.Archive.SetEnabled(isDetailView)
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
		m.table.SetHeight(m.height - 8)
		m.help.Width = m.width
		return m, nil

	case tea.KeyMsg:
		if m.state == detailView {
			switch {
			case key.Matches(msg, m.keys.Back):
				if m.detailContent != "" {
					m.detailContent = "" // First back press clears content
					m.notification = ""
				} else {
					m.state = listView // Second back press returns to list
					m.selectedSession = nil
					m.notification = ""
					m.updateKeyBindings()
				}
				return m, nil
			case key.Matches(msg, m.keys.Log):
				if m.selectedSession != nil {
					return m, getSessionLogsCmd(m.sessionManager, m.selectedSession.Name)
				}
			case key.Matches(msg, m.keys.Diff):
				if m.selectedSession != nil {
					return m, getSessionDiffCmd(m.sessionManager, m.selectedSession.Name)
				}
			case key.Matches(msg, m.keys.Stop):
				if m.selectedSession != nil {
					return m, stopSessionCmd(m.sessionManager, m.selectedSession.Name)
				}
			case key.Matches(msg, m.keys.Archive):
				if m.selectedSession != nil {
					return m, archiveSessionCmd(m.sessionManager, m.selectedSession.Name)
				}
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			}
		} else { // listView
			switch {
			case key.Matches(msg, m.keys.Enter):
				if len(m.sessions) > 0 && m.table.Cursor() < len(m.sessions) {
					m.selectedSession = &m.sessions[m.table.Cursor()]
					m.state = detailView
					m.updateKeyBindings()
				}
				return m, nil
			case key.Matches(msg, m.keys.Quit):
				return m, tea.Quit
			}
		}

	case psTickMsg:
		return m, refreshPsSessionsCmd()

	case psSessionsRefreshedMsg:
		m.sessions = msg
		m.lastUpdate = time.Now()
		m.updateTableRows()
		return m, nil

	case notificationMsg:
		m.notification = string(msg)
		return m, nil

	case detailContentMsg:
		m.detailContent = string(msg)
		return m, nil

	case error:
		m.err = msg
		return m, nil
	}

	if m.state == listView {
		m.table, cmd = m.table.Update(msg)
	}
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

	switch m.state {
	case detailView:
		return m.detailView()
	default: // listView
		return m.listView()
	}
}

func (m psDashboardModel) listView() string {
	var s strings.Builder
	s.WriteString(psDashboardTitleStyle.Render(" RECAC Session Explorer") + "\n")
	s.WriteString(fmt.Sprintf("Last updated: %s\n\n", m.lastUpdate.Format(time.RFC1123)))
	s.WriteString(m.table.View())
	s.WriteString("\n" + m.help.View(m.keys))
	return s.String()
}

func (m psDashboardModel) detailView() string {
	if m.selectedSession == nil {
		return "Error: No session selected"
	}

	if m.detailContent != "" {
		// If there's content (logs/diff), show only that
		return m.detailContent + "\n\n" + m.help.View(m.keys)
	}

	s := m.selectedSession
	var b strings.Builder

	b.WriteString(psDashboardTitleStyle.Render(fmt.Sprintf(" Details for %s", s.Name)) + "\n\n")
	b.WriteString(psDetailStyle.Render(fmt.Sprintf("Status:     %s\n", s.Status)))
	b.WriteString(psDetailStyle.Render(fmt.Sprintf("Location:   %s\n", s.Location)))
	b.WriteString(psDetailStyle.Render(fmt.Sprintf("Started:    %s\n", s.StartTime.Format(time.RFC1123))))
	b.WriteString(psDetailStyle.Render(fmt.Sprintf("Last Used:  %s\n", formatSince(s.LastActivity))))
	if s.HasCost {
		b.WriteString(psDetailStyle.Render(fmt.Sprintf("Cost:       $%.6f\n", s.Cost)))
		b.WriteString(psDetailStyle.Render(fmt.Sprintf("Tokens:     %d (Prompt: %d, Completion: %d)\n", s.Tokens.TotalTokens, s.Tokens.TotalPromptTokens, s.Tokens.TotalResponseTokens)))
	}
	b.WriteString(psDetailStyle.Render(fmt.Sprintf("\nGoal:\n%s\n", s.Goal)))

	if m.notification != "" {
		b.WriteString("\n" + lipgloss.NewStyle().Foreground(lipgloss.Color("62")).Render(m.notification))
	}

	b.WriteString("\n\n" + m.help.View(m.keys))
	return b.String()
}

func getSessionLogsCmd(sm ISessionManagerDashboard, name string) tea.Cmd {
	return func() tea.Msg {
		logs, err := sm.GetSessionLogs(name)
		if err != nil {
			return notificationMsg(fmt.Sprintf("Error getting logs: %v", err))
		}
		return detailContentMsg(logs)
	}
}

func getSessionDiffCmd(sm ISessionManagerDashboard, name string) tea.Cmd {
	return func() tea.Msg {
		diff, err := sm.GetSessionDiff(name)
		if err != nil {
			return notificationMsg(fmt.Sprintf("Error getting diff: %v", err))
		}
		return detailContentMsg(diff)
	}
}

func stopSessionCmd(sm ISessionManagerDashboard, name string) tea.Cmd {
	return func() tea.Msg {
		err := sm.StopSession(name)
		if err != nil {
			return notificationMsg(fmt.Sprintf("Error stopping session: %v", err))
		}
		return notificationMsg(fmt.Sprintf("Session '%s' stopped.", name))
	}
}

func archiveSessionCmd(sm ISessionManagerDashboard, name string) tea.Cmd {
	return func() tea.Msg {
		err := sm.ArchiveSession(name)
		if err != nil {
			return notificationMsg(fmt.Sprintf("Error archiving session: %v", err))
		}
		return notificationMsg(fmt.Sprintf("Session '%s' archived.", name))
	}
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

var StartPsDashboard = func(sm ISessionManagerDashboard) error {
	p := tea.NewProgram(NewPsDashboardModel(sm), tea.WithAltScreen())
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
