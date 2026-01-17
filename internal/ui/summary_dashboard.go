package ui

import (
	"fmt"
	"recac/internal/agent"
	"recac/internal/runner"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type summaryModel struct {
	sessions   []*runner.SessionState
	lastUpdate time.Time
	err        error
}

type summaryTickMsg time.Time
type sessionsRefreshedMsg []*runner.SessionState

var (
	summaryTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	summaryHeaderStyle      = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("242"))
	cellStyle        = lipgloss.NewStyle().Padding(0, 1)
)

func NewSummaryModel() summaryModel {
	return summaryModel{}
}

func (m summaryModel) Init() tea.Cmd {
	return tea.Batch(refreshSessionsCmd(), tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return summaryTickMsg(t)
	}))
}

func (m summaryModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case summaryTickMsg:
		return m, refreshSessionsCmd()
	case sessionsRefreshedMsg:
		m.sessions = msg
		m.lastUpdate = time.Now()
		return m, nil
	case error:
		m.err = msg
		return m, nil
	}
	return m, nil
}

func (m summaryModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var s strings.Builder
	s.WriteString(summaryTitleStyle.Render("📊 RECAC Summary Dashboard") + "\n")
	s.WriteString(fmt.Sprintf("Last updated: %s (press 'q' to quit)\n", m.lastUpdate.Format(time.RFC1123)))

	if len(m.sessions) == 0 {
		s.WriteString("\nNo sessions found.")
		return s.String()
	}

	// Calculate stats
	var totalTokens int
	var totalCost float64
	completed, errored, running := 0, 0, 0
	sessionCosts := make(map[string]float64)

	for _, session := range m.sessions {
		switch session.Status {
		case "completed":
			completed++
		case "error":
			errored++
		case "running":
			running++
		}
		if session.AgentStateFile != "" {
			state, err := agent.LoadState(session.AgentStateFile)
			if err == nil {
				cost := agent.CalculateCost(state.Model, state.TokenUsage)
				totalCost += cost
				totalTokens += state.TokenUsage.TotalTokens
				sessionCosts[session.Name] = cost
			}
		}
	}

	// Render stats
	s.WriteString(m.renderStats(len(m.sessions), completed, errored, running, totalTokens, totalCost))

	// Render recent sessions
	s.WriteString(m.renderRecentSessions())

	// Render most expensive sessions
	s.WriteString(m.renderMostExpensiveSessions(sessionCosts))

	return s.String()
}

func (m *summaryModel) renderStats(total, completed, errored, running, totalTokens int, totalCost float64) string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\n"+summaryHeaderStyle.Render("Aggregate Stats"))
	fmt.Fprintln(w, "-------------------")
	fmt.Fprintf(w, "Total Sessions:\t%d\n", total)
	fmt.Fprintf(w, "Completed:\t%d\n", completed)
	fmt.Fprintf(w, "Errored:\t%d\n", errored)
	fmt.Fprintf(w, "Running:\t%d\n", running)
	if total > 0 {
		successRate := float64(completed) / float64(total) * 100
		fmt.Fprintf(w, "Success Rate:\t%.2f%%\n", successRate)
	}
	fmt.Fprintf(w, "Total Tokens:\t%d\n", totalTokens)
	fmt.Fprintf(w, "Total Est. Cost:\t$%.4f\n", totalCost)
	w.Flush()
	return b.String()
}

func (m *summaryModel) renderRecentSessions() string {
	sort.Slice(m.sessions, func(i, j int) bool {
		return m.sessions[i].StartTime.After(m.sessions[j].StartTime)
	})

	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\n"+summaryHeaderStyle.Render("Recent Sessions (Top 5)"))
	fmt.Fprintln(w, "-------------------------")
	fmt.Fprintln(w, "NAME\tSTATUS\tSTART TIME\tDURATION")
	for i, s := range m.sessions {
		if i >= 5 {
			break
		}
		duration := time.Since(s.StartTime).Round(time.Second)
		if !s.EndTime.IsZero() {
			duration = s.EndTime.Sub(s.StartTime).Round(time.Second)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", s.Name, s.Status, s.StartTime.Format(time.RFC3339), duration)
	}
	w.Flush()
	return b.String()
}

func (m *summaryModel) renderMostExpensiveSessions(sessionCosts map[string]float64) string {
	sort.Slice(m.sessions, func(i, j int) bool {
		return sessionCosts[m.sessions[i].Name] > sessionCosts[m.sessions[j].Name]
	})

	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "\n"+summaryHeaderStyle.Render("Most Expensive Sessions (Top 5)"))
	fmt.Fprintln(w, "---------------------------------")
	fmt.Fprintln(w, "NAME\tCOST\tTOKENS\tMODEL")
	for i, s := range m.sessions {
		if i >= 5 {
			break
		}
		cost := sessionCosts[s.Name]
		if cost == 0 {
			continue
		}
		tokens := 0
		model := "N/A"
		if s.AgentStateFile != "" {
			state, err := agent.LoadState(s.AgentStateFile)
			if err == nil {
				tokens = state.TokenUsage.TotalTokens
				model = state.Model
			}
		}
		fmt.Fprintf(w, "%s\t$%.4f\t%d\t%s\n", s.Name, cost, tokens, model)
	}
	w.Flush()
	return b.String()
}


func refreshSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		sm, err := runner.NewSessionManager()
		if err != nil {
			return err
		}
		sessions, err := sm.ListSessions()
		if err != nil {
			return err
		}
		return sessionsRefreshedMsg(sessions)
	}
}


func StartSummaryDashboard() error {
	p := tea.NewProgram(NewSummaryModel())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
