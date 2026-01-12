package ui

import (
	"fmt"
	"os"
	"recac/internal/agent"
	"recac/internal/runner"
	"sort"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/help"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	dashboardAppStyle = lipgloss.NewStyle().Margin(1, 2)
	headerStyle       = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("228")). // Yellow
				Padding(0, 1)
	sessionListStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("63")). // Purple
				Padding(1, 2)
	detailsStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")). // Grey
			Padding(1, 2)
	logViewStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")). // Grey
			Padding(1, 2)
	footerStyle = lipgloss.NewStyle().MarginTop(1)
)

// KeyMap defines the keybindings for the dashboard.
type dashboardKeyMap struct {
	Quit key.Binding
	Up   key.Binding
	Down key.Binding
}

func (k dashboardKeyMap) ShortHelp() []key.Binding {
	return []key.Binding{k.Up, k.Down, k.Quit}
}

func (k dashboardKeyMap) FullHelp() [][]key.Binding {
	return [][]key.Binding{
		{k.Up, k.Down},
		{k.Quit},
	}
}

var dashboardKeys = dashboardKeyMap{
	Up: key.NewBinding(
		key.WithKeys("up", "k"),
		key.WithHelp("↑/k", "move up"),
	),
	Down: key.NewBinding(
		key.WithKeys("down", "j"),
		key.WithHelp("↓/j", "move down"),
	),
	Quit: key.NewBinding(
		key.WithKeys("q", "ctrl+c"),
		key.WithHelp("q/ctrl+c", "quit"),
	),
}

// Model represents the state of the dashboard TUI.
type DashboardModel struct {
	keys    dashboardKeyMap
	help    help.Model
	spinner spinner.Model

	sessionManager ISessionManager // Dependency
	sessionList    list.Model
	detailsView    viewport.Model
	logView        viewport.Model

	sessions []*runner.SessionState // Cache for session data
	loading  bool
	width    int
	height   int
	err      error
}

// sessionItem represents a session in the list.
type sessionItem struct {
	name   string
	status string
	id     string // Usually the same as name
}

func (i sessionItem) Title() string       { return i.name }
func (i sessionItem) Description() string { return "Status: " + i.status }
func (i sessionItem) FilterValue() string { return i.name }

// NewDashboardModel creates a new dashboard model.
func NewDashboardModel(sm ISessionManager) DashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	// Create a placeholder list for now
	items := []list.Item{}
	delegate := list.NewDefaultDelegate()
	sessionList := list.New(items, delegate, 0, 0)
	sessionList.Title = "RECAC Sessions"
	sessionList.Styles.Title = interactiveTitleStyle

	detailsView := viewport.New(0, 0)
	logView := viewport.New(0, 0)

	return DashboardModel{
		keys:           dashboardKeys,
		help:           help.New(),
		spinner:        s,
		sessionManager: sm,
		sessionList:    sessionList,
		detailsView:    detailsView,
		logView:        logView,
		loading:        true,
	}
}

// Init is the first command that will be executed.
func (m DashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		tea.EnterAltScreen,
		refreshSessionsCmd(m.sessionManager), // Initial data load
		tickCmd(),                            // Start the refresh timer
	)
}

// --- Bubble Tea Commands & Messages ---

type sessionsRefreshedMsg struct {
	sessions []*runner.SessionState
}
type refreshTickMsg time.Time

func refreshSessionsCmd(sm ISessionManager) tea.Cmd {
	return func() tea.Msg {
		sessions, err := sm.ListSessions()
		if err != nil {
			// In a real app, you'd have an error message type
			return nil
		}
		// Sort by start time, newest first
		sort.SliceStable(sessions, func(i, j int) bool {
			return sessions[i].StartTime.After(sessions[j].StartTime)
		})
		return sessionsRefreshedMsg{sessions: sessions}
	}
}

// tickCmd creates a command that sends a tick message every 5 seconds.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*5, func(t time.Time) tea.Msg {
		return refreshTickMsg(t)
	})
}

// Update handles all incoming messages.
func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.help.Width = msg.Width

		listWidth := m.width / 3
		detailsWidth := m.width - listWidth
		m.sessionList.SetSize(listWidth-4, m.height-6)
		m.detailsView.Width = detailsWidth - 4
		m.detailsView.Height = (m.height / 2) - 5
		m.logView.Width = detailsWidth - 4
		m.logView.Height = (m.height / 2) - 5

	case sessionsRefreshedMsg:
		m.loading = false
		m.sessions = msg.sessions
		items := make([]list.Item, len(m.sessions))
		for i, s := range m.sessions {
			items[i] = sessionItem{name: s.Name, status: s.Status, id: s.Name}
		}
		m.sessionList.SetItems(items)

	case refreshTickMsg:
		// Trigger a refresh and schedule the next tick
		return m, tea.Batch(refreshSessionsCmd(m.sessionManager), tickCmd())

	case tea.KeyMsg:
		if m.sessionList.FilterState() == list.Filtering {
			break
		}
		switch {
		case key.Matches(msg, m.keys.Quit):
			return m, tea.Quit
		}
	}

	// Update components
	var listCmd, spinnerCmd, detailsCmd, logCmd tea.Cmd
	m.sessionList, listCmd = m.sessionList.Update(msg)
	m.spinner, spinnerCmd = m.spinner.Update(msg)
	m.detailsView, detailsCmd = m.detailsView.Update(msg)
	m.logView, logCmd = m.logView.Update(msg)
	cmds = append(cmds, listCmd, spinnerCmd, detailsCmd, logCmd)

	// Update details view based on selection
	m.updateDetailsView()

	return m, tea.Batch(cmds...)
}

// View renders the TUI.
func (m DashboardModel) View() string {
	if m.loading || m.width == 0 {
		return fmt.Sprintf("\n  %s Loading session data...", m.spinner.View())
	}

	// Header
	header := headerStyle.Render(fmt.Sprintf("RECAC Dashboard (%d sessions)", len(m.sessions)))

	// Main content
	sessionListView := sessionListStyle.Render(m.sessionList.View())
	detailsView := detailsStyle.Render(m.detailsView.View())
	logView := logViewStyle.Render(m.logView.View())

	rightPanel := lipgloss.JoinVertical(lipgloss.Left, detailsView, logView)
	mainContent := lipgloss.JoinHorizontal(lipgloss.Top, sessionListView, rightPanel)

	// Footer
	helpView := m.help.View(m.keys)
	footer := footerStyle.Render(helpView)

	return lipgloss.JoinVertical(lipgloss.Left,
		header,
		mainContent,
		footer,
	)
}

// --- Data Loading Abstraction ---
// This is a bit of a workaround to avoid a direct dependency on cmd/recac from internal/ui
// In a larger refactor, the data loading functions would move to a shared package.
var loadAgentState = func(filePath string) (*agent.State, error) {
	// This function will be replaced by the real implementation from main.go
	return nil, fmt.Errorf("loadAgentState not implemented")
}

// SetAgentStateLoader allows the main package to inject the implementation.
func SetAgentStateLoader(loader func(string) (*agent.State, error)) {
	loadAgentState = loader
}

func (i sessionItem) Title() string       { return i.name }
func (i sessionItem) Description() string { return "Status: " + i.status }
func (i sessionItem) FilterValue() string { return i.name }

func (m *DashboardModel) updateDetailsView() {
	selectedItem, ok := m.sessionList.SelectedItem().(sessionItem)
	if !ok {
		m.detailsView.SetContent("Select a session to see details.")
		m.logView.SetContent("")
		return
	}

	var selectedSession *runner.SessionState
	for _, s := range m.sessions {
		if s.Name == selectedItem.id {
			selectedSession = s
			break
		}
	}

	if selectedSession == nil {
		m.detailsView.SetContent(fmt.Sprintf("Error: Could not find session '%s'", selectedItem.id))
		return
	}

	// Build details string
	var b strings.Builder
	fmt.Fprintf(&b, "Name: %s\n", selectedSession.Name)
	fmt.Fprintf(&b, "Status: %s\n", selectedSession.Status)
	fmt.Fprintf(&b, "PID: %d\n", selectedSession.PID)
	fmt.Fprintf(&b, "Start: %s\n", selectedSession.StartTime.Format(time.RFC822))
	if !selectedSession.EndTime.IsZero() {
		fmt.Fprintf(&b, "Duration: %s\n", selectedSession.EndTime.Sub(selectedSession.StartTime).Round(time.Second))
	} else {
		fmt.Fprintf(&b, "Duration: %s\n", time.Since(selectedSession.StartTime).Round(time.Second))
	}

	// Load and display agent state (cost, tokens)
	agentState, err := loadAgentState(selectedSession.AgentStateFile)
	if err == nil && agentState != nil {
		cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)
		fmt.Fprintf(&b, "\nModel: %s\n", agentState.Model)
		fmt.Fprintf(&b, "Tokens: %d\n", agentState.TokenUsage.TotalTokens)
		fmt.Fprintf(&b, "Cost: $%.4f\n", cost)
	}

	m.detailsView.SetContent(b.String())

	// Update log view
	if logContent, err := os.ReadFile(selectedSession.LogFile); err == nil {
		m.logView.SetContent(string(logContent))
	} else {
		m.logView.SetContent(fmt.Sprintf("Could not read log file:\n%s", err.Error()))
	}
}
