package ui

import (
	"fmt"
	"os"
	"time"

	"recac/internal/agent"
	"recac/internal/runner"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// SessionManager is an interface that defines the methods needed by the cost TUI.
// This is defined locally to avoid circular dependencies with the cmd package.
type SessionManager interface {
	ListSessions() ([]*runner.SessionState, error)
}

// LoadAgentStateFunc defines the signature for a function that can load agent state.
// This allows the cmd package to inject its own implementation.
type LoadAgentStateFunc func(filePath string) (*agent.State, error)

var (
	// LoadAgentState is the function used to load the agent state.
	// This must be set by the calling package (e.g., cmd) before starting the TUI.
	LoadAgentState LoadAgentStateFunc

	baseStyle = lipgloss.NewStyle().
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(lipgloss.Color("240"))
)

var startCostTUI = func(sm SessionManager) error {
	if LoadAgentState == nil {
		return fmt.Errorf("LoadAgentState function must be set before starting the Cost TUI")
	}

	model := newCostModel(sm)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if err := p.Start(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		return err
	}
	return nil
}

// StartCostTUI is a wrapper that can be mocked for testing.
func StartCostTUI(sm SessionManager) error {
	return startCostTUI(sm)
}

// SetStartCostTUIForTest allows tests to replace the TUI starter function.
func SetStartCostTUIForTest(fn func(sm SessionManager) error) {
	startCostTUI = fn
}

type costModel struct {
	table    table.Model
	sm       SessionManager
	sessions []*runner.SessionState
	err      error
}

type updateMsg []*runner.SessionState
type errMsg struct{ err error }

func newCostModel(sm SessionManager) *costModel {
	columns := []table.Column{
		{Title: "NAME", Width: 20},
		{Title: "STATUS", Width: 10},
		{Title: "STARTED", Width: 20},
		{Title: "DURATION", Width: 15},
		{Title: "PROMPT", Width: 10},
		{Title: "COMPLETION", Width: 10},
		{Title: "TOTAL", Width: 10},
		{Title: "COST", Width: 15},
	}

	rows := []table.Row{}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(15), // Increased height for better view
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(true) // Bolder header
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(false)
	t.SetStyles(s)

	return &costModel{
		table: t,
		sm:    sm,
	}
}

func (m *costModel) Init() tea.Cmd {
	return tea.Batch(tickCmd(), fetchSessions(m.sm))
}

func (m *costModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tickMsg:
		return m, tea.Batch(tickCmd(), fetchSessions(m.sm))
	case updateMsg:
		m.sessions = msg
		m.updateTable()
		return m, nil
	case errMsg:
		m.err = msg.err
		return m, nil
	case tea.WindowSizeMsg:
		// Adjust table height on window resize
		m.table.SetHeight(msg.Height - 5)
		return m, nil
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *costModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n\nPress 'q' to quit.", m.err)
	}
	title := " RECAC Live Session Monitor "
	help := "\n  ↑/↓: Navigate • q: Quit"
	return baseStyle.Render(title+"\n"+m.table.View()) + help
}

func (m *costModel) updateTable() {
	rows := make([]table.Row, len(m.sessions))
	for i, session := range m.sessions {
		started := session.StartTime.Format("2006-01-02 15:04:05")
		var duration string
		if session.EndTime.IsZero() {
			duration = time.Since(session.StartTime).Round(time.Second).String()
		} else {
			duration = session.EndTime.Sub(session.StartTime).Round(time.Second).String()
		}

		agentState, err := LoadAgentState(session.AgentStateFile)
		if err != nil || agentState == nil {
			rows[i] = table.Row{
				session.Name,
				session.Status,
				started,
				duration,
				"N/A",
				"N/A",
				"N/A",
				"N/A",
			}
		} else {
			cost := agent.CalculateCost(agentState.Model, agentState.TokenUsage)
			rows[i] = table.Row{
				session.Name,
				session.Status,
				started,
				duration,
				fmt.Sprintf("%d", agentState.TokenUsage.TotalPromptTokens),
				fmt.Sprintf("%d", agentState.TokenUsage.TotalResponseTokens),
				fmt.Sprintf("%d", agentState.TokenUsage.TotalTokens),
				fmt.Sprintf("$%.6f", cost),
			}
		}
	}
	m.table.SetRows(rows)
}

type tickMsg time.Time

func tickCmd() tea.Cmd {
	return tea.Tick(time.Second*2, func(t time.Time) tea.Msg { // Slower tick for less CPU usage
		return tickMsg(t)
	})
}

func fetchSessions(sm SessionManager) tea.Cmd {
	return func() tea.Msg {
		sessions, err := sm.ListSessions()
		if err != nil {
			return errMsg{err}
		}
		return updateMsg(sessions)
	}
}
