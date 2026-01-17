package ui

import (
	"fmt"
	"recac/internal/agent"
	"recac/internal/model"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Injected dependencies
var (
	GetSessionDetail func(sessionName string) (*model.UnifiedSession, error)
	GetSessionLogs   func(sessionName string) (string, error)
	GetAgentState    func(sessionName string) (*agent.State, error)
)

type SessionDashboardModel struct {
	sessionName string
	session     *model.UnifiedSession
	agentState  *agent.State
	logs        string
	err         error

	// Viewports
	logsViewport    viewport.Model
	historyViewport viewport.Model

	width  int
	height int

	// Ticks
	lastUpdate time.Time
}

type dashboardTickMsg time.Time
type sessionDataMsg struct {
	session *model.UnifiedSession
	state   *agent.State
	logs    string
}

// Styles
var (
	dashboardTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#7D56F4")).
			Padding(0, 1)

	paneStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#874BFD")).
			Padding(0, 1)

	dashboardHeaderStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FAFAFA")).
			Padding(0, 1)

	subtleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#767676"))

	logStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#00FF00")) // Classic green terminal for logs
)

func NewSessionDashboardModel(sessionName string) SessionDashboardModel {
	m := SessionDashboardModel{
		sessionName: sessionName,
	}

	m.logsViewport = viewport.New(0, 0)
	m.logsViewport.Style = lipgloss.NewStyle().Padding(0, 1)

	m.historyViewport = viewport.New(0, 0)
	m.historyViewport.Style = lipgloss.NewStyle().Padding(0, 1)

	return m
}

func (m SessionDashboardModel) Init() tea.Cmd {
	// Start polling loop. The update loop will schedule the next tick.
	return refreshSessionDataCmd(m.sessionName)
}

func (m SessionDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		// Layout calculations
		headerHeight := 6
		footerHeight := 2
		availableHeight := m.height - headerHeight - footerHeight
		halfWidth := (m.width / 2) - 4 // border adjustment

		m.logsViewport.Width = halfWidth
		m.logsViewport.Height = availableHeight

		m.historyViewport.Width = halfWidth
		m.historyViewport.Height = availableHeight

		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case dashboardTickMsg:
		return m, refreshSessionDataCmd(m.sessionName)

	case sessionDataMsg:
		m.session = msg.session
		m.agentState = msg.state
		m.logs = msg.logs
		m.lastUpdate = time.Now()

		m.logsViewport.SetContent(logStyle.Render(m.logs))
		// Only auto-scroll if we are at the bottom or it's the first load
		if m.logsViewport.AtBottom() || m.lastUpdate.IsZero() {
			m.logsViewport.GotoBottom()
		}

		if m.agentState != nil && len(m.agentState.History) > 0 {
			formattedHistory := formatHistory(m.agentState.History)
			m.historyViewport.SetContent(formattedHistory)
			if m.historyViewport.AtBottom() || m.lastUpdate.IsZero() {
				m.historyViewport.GotoBottom()
			}
		} else {
			m.historyViewport.SetContent("Waiting for agent activity...")
		}

		return m, tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return dashboardTickMsg(t)
		})

	case error:
		m.err = msg
		return m, nil
	}

	m.logsViewport, cmd = m.logsViewport.Update(msg)
	cmds = append(cmds, cmd)

	m.historyViewport, cmd = m.historyViewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

func (m SessionDashboardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}
	if m.session == nil {
		return "Loading session data..."
	}

	// 1. Header
	header := m.renderHeader()

	// 2. Panes
	logsPane := paneStyle.
		Width(m.logsViewport.Width).
		Height(m.logsViewport.Height).
		Render(
			lipgloss.JoinVertical(lipgloss.Left,
				dashboardTitleStyle.Background(lipgloss.Color("#0000AA")).Render("Live Logs"),
				m.logsViewport.View(),
			),
		)

	historyPane := paneStyle.
		Width(m.historyViewport.Width).
		Height(m.historyViewport.Height).
		Render(
			lipgloss.JoinVertical(lipgloss.Left,
				dashboardTitleStyle.Background(lipgloss.Color("#AA00AA")).Render("Agent Thought Stream"),
				m.historyViewport.View(),
			),
		)

	// Combine Panes
	panes := lipgloss.JoinHorizontal(lipgloss.Top, logsPane, historyPane)

	// 3. Footer
	footer := subtleStyle.Render(fmt.Sprintf("Last updated: %s | Press 'q' to quit", m.lastUpdate.Format("15:04:05")))

	return lipgloss.JoinVertical(lipgloss.Left, header, panes, footer)
}

func (m SessionDashboardModel) renderHeader() string {
	s := m.session
	statusColor := "#00FF00"
	if s.Status == "error" {
		statusColor = "#FF0000"
	}

	statusStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(statusColor)).Bold(true)

	left := lipgloss.JoinVertical(lipgloss.Left,
		dashboardHeaderStyle.Render(fmt.Sprintf("SESSION: %s", dashboardTitleStyle.Render(s.Name))),
		dashboardHeaderStyle.Render(fmt.Sprintf("GOAL: %s", truncate(s.Goal, 80))),
	)

	right := lipgloss.JoinVertical(lipgloss.Left,
		dashboardHeaderStyle.Render(fmt.Sprintf("STATUS: %s | CPU: %s | MEM: %s", statusStyle.Render(strings.ToUpper(s.Status)), s.CPU, s.Memory)),
		dashboardHeaderStyle.Render(fmt.Sprintf("TOKENS: %d ($%.4f) | MODEL: %s", s.Tokens.TotalTokens, s.Cost, m.getAgentModel())),
	)

	return lipgloss.JoinHorizontal(lipgloss.Top,
		lipgloss.NewStyle().Width(m.width/2).Render(left),
		lipgloss.NewStyle().Width(m.width/2).Align(lipgloss.Right).Render(right),
	)
}

func (m SessionDashboardModel) getAgentModel() string {
	if m.agentState != nil {
		return m.agentState.Model
	}
	return "unknown"
}

func refreshSessionDataCmd(name string) tea.Cmd {
	return func() tea.Msg {
		if GetSessionDetail == nil {
			return fmt.Errorf("GetSessionDetail not injected")
		}

		s, err := GetSessionDetail(name)
		if err != nil {
			return err
		}

		var state *agent.State
		if GetAgentState != nil {
			state, _ = GetAgentState(name)
		}

		logs := ""
		if GetSessionLogs != nil {
			logs, _ = GetSessionLogs(name)
		}

		return sessionDataMsg{session: s, state: state, logs: logs}
	}
}

func formatHistory(history []agent.Message) string {
	var b strings.Builder
	for _, msg := range history {
		roleStyle := lipgloss.NewStyle().Bold(true)
		if msg.Role == "user" {
			roleStyle = roleStyle.Foreground(lipgloss.Color("#00FFFF")) // Cyan
			b.WriteString(roleStyle.Render("USER > ") + msg.Content + "\n\n")
		} else if msg.Role == "assistant" {
			roleStyle = roleStyle.Foreground(lipgloss.Color("#FF00FF")) // Magenta
			// Try to extract "Thought" if present in XML-like tags (common in some prompts)
			// otherwise just print content
			b.WriteString(roleStyle.Render("AGENT > ") + msg.Content + "\n\n")
		} else {
			b.WriteString(fmt.Sprintf("[%s] %s\n\n", msg.Role, msg.Content))
		}
	}
	return b.String()
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + "..."
	}
	return s
}

// Public entry point
var StartSessionDashboard = func(sessionName string) error {
	p := tea.NewProgram(NewSessionDashboardModel(sessionName), tea.WithAltScreen())
	_, err := p.Run()
	return err
}
