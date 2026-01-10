package main

import (
	"fmt"
	"os"
	"time"

	"recac/internal/runner"
	"recac/internal/agent"


	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(topCmd)
}

var topCmd = &cobra.Command{
	Use:   "top",
	Short: "Monitor running sessions in real-time",
	Long:  `Provides a real-time, dynamic view of active sessions, similar to htop.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		sm, err := sessionManagerFactory()
		if err != nil {
			return fmt.Errorf("failed to create session manager: %w", err)
		}

		model := newTopModel(sm)
		p := tea.NewProgram(model, tea.WithAltScreen())

		if err := p.Start(); err != nil {
			fmt.Fprintf(os.Stderr, "Error running program: %v\n", err)
			return err
		}
		return nil
	},
}

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type topModel struct {
	table    table.Model
	sm       ISessionManager
	sessions []*runner.SessionState
	err      error
}

type updateMsg []*runner.SessionState
type errMsg struct{ err error }

func newTopModel(sm ISessionManager) *topModel {
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
		table.WithHeight(7),
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

	return &topModel{
		table: t,
		sm:    sm,
	}
}

func (m *topModel) Init() tea.Cmd {
	return tea.Batch(tickCmd(), fetchSessions(m.sm))
}

func (m *topModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m *topModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v\n", m.err)
	}
	return baseStyle.Render(m.table.View()) + "\n"
}

func (m *topModel) updateTable() {
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
		if err != nil {
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
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func fetchSessions(sm ISessionManager) tea.Cmd {
	return func() tea.Msg {
		sessions, err := sm.ListSessions()
		if err != nil {
			return errMsg{err}
		}
		return updateMsg(sessions)
	}
}
