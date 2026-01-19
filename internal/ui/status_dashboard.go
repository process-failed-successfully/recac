package ui

import (
	"fmt"
	"recac/internal/agent"
	"recac/internal/runner"
	"recac/internal/utils"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// GetSessionStatusFunc is the signature for the data fetching function
type GetSessionStatusFunc func(sessionName string) (*runner.SessionState, *agent.State, string, error)

// GetSessionStatus will be set by the caller in the cmd package
var GetSessionStatus GetSessionStatusFunc

type statusDashboardModel struct {
	sessionName string
	session     *runner.SessionState
	agentState  *agent.State
	gitDiffStat string
	lastUpdate  time.Time
	err         error
	width       int
	height      int
	spinner     spinner.Model
}

type statusTickMsg time.Time
type statusRefreshedMsg struct {
	session     *runner.SessionState
	agentState  *agent.State
	gitDiffStat string
}

var (
	statusTitleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("62"))
	labelStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("242")).Bold(true)
	valueStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("252"))
	sectionStyle     = lipgloss.NewStyle().MarginTop(1).Foreground(lipgloss.Color("86")).Bold(true)
	statusHelpStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).MarginTop(2)
)

func NewStatusDashboardModel(sessionName string) statusDashboardModel {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))

	return statusDashboardModel{
		sessionName: sessionName,
		spinner:     s,
	}
}

func (m statusDashboardModel) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		refreshStatusCmd(m.sessionName),
		tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
			return statusTickMsg(t)
		}),
	)
}

func (m statusDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}

	case spinner.TickMsg:
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	case statusTickMsg:
		return m, tea.Batch(
			refreshStatusCmd(m.sessionName),
			tea.Tick(1*time.Second, func(t time.Time) tea.Msg {
				return statusTickMsg(t)
			}),
		)

	case statusRefreshedMsg:
		m.session = msg.session
		m.agentState = msg.agentState
		m.gitDiffStat = msg.gitDiffStat
		m.lastUpdate = time.Now()
		return m, nil

	case error:
		m.err = msg
		return m, nil
	}

	return m, nil
}

func (m statusDashboardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var s strings.Builder

	// Header with spinner
	headerText := fmt.Sprintf(" RECAC Session Status: %s", m.sessionName)
	s.WriteString(fmt.Sprintf("%s %s\n", m.spinner.View(), statusTitleStyle.Render(headerText)))

	if m.session == nil {
		s.WriteString("\n  Initializing session monitor...\n")
		return s.String()
	}

	s.WriteString(fmt.Sprintf("Last updated: %s\n\n", m.lastUpdate.Format(time.RFC1123)))

	// --- Session Info ---
	s.WriteString(renderSessionInfo(m.session, m.agentState))

	// --- Usage Info ---
	if m.agentState != nil {
		s.WriteString(renderUsageInfo(m.agentState))
	} else {
		s.WriteString("\nAgent state not available.\n")
	}

	// --- Git Changes ---
	if m.gitDiffStat != "" {
		s.WriteString(sectionStyle.Render("\n--- Git Changes ---") + "\n")
		s.WriteString(m.gitDiffStat + "\n")
	}

	// --- Last Activity ---
	if m.agentState != nil {
		s.WriteString(renderLastActivity(m.agentState, m.width))
	}

	// --- Help Footer ---
	s.WriteString(statusHelpStyle.Render("Press 'q' to quit • Updates automatically"))

	return s.String()
}

func renderSessionInfo(s *runner.SessionState, a *agent.State) string {
	var b strings.Builder

	statusColor := lipgloss.Color("252")
	statusIcon := "•"

	switch strings.ToLower(s.Status) {
	case "running":
		statusColor = lipgloss.Color("46") // Green
		statusIcon = "⚡"
	case "completed":
		statusColor = lipgloss.Color("39") // Blue
		statusIcon = "✔"
	case "error", "failed":
		statusColor = lipgloss.Color("196") // Red
		statusIcon = "✖"
	}
	statusStyle := lipgloss.NewStyle().Foreground(statusColor).Bold(true)

	b.WriteString(fmt.Sprintf("%s %s %s\n", labelStyle.Render("Status:"), statusStyle.Render(statusIcon), statusStyle.Render(s.Status)))

	goal := s.Goal
	if len(goal) > 80 {
		goal = goal[:77] + "..."
	}
	b.WriteString(fmt.Sprintf("%s   %s\n", labelStyle.Render("Goal:"), valueStyle.Render(goal)))

	modelName := "N/A"
	if a != nil {
		modelName = a.Model
	}
	b.WriteString(fmt.Sprintf("%s  %s\n", labelStyle.Render("Model:"), valueStyle.Render(modelName)))

	duration := utils.FormatSince(s.StartTime)
	if !s.EndTime.IsZero() {
		duration = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
	}
	b.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Time:"), valueStyle.Render(fmt.Sprintf("%s (%s ago)", s.StartTime.Format(time.RFC822), duration))))

	return b.String()
}

func renderUsageInfo(state *agent.State) string {
	var b strings.Builder
	b.WriteString(sectionStyle.Render("\n--- Usage ---") + "\n")

	cost := agent.CalculateCost(state.Model, state.TokenUsage)
	costStr := fmt.Sprintf("$%.6f", cost)
	if cost > 0.5 {
		// Add warning icon for high cost
		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true)
		costStr = warningStyle.Render("⚠ " + costStr)
	} else {
		costStr = valueStyle.Render(costStr)
	}

	b.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Tokens:"), valueStyle.Render(fmt.Sprintf("%d (P: %d, C: %d)",
		state.TokenUsage.TotalTokens,
		state.TokenUsage.TotalPromptTokens,
		state.TokenUsage.TotalResponseTokens))))

	b.WriteString(fmt.Sprintf("%s   %s\n", labelStyle.Render("Cost:"), costStr))
	return b.String()
}

func renderLastActivity(state *agent.State, width int) string {
	var b strings.Builder
	if len(state.History) > 0 {
		lastMessage := state.History[len(state.History)-1]
		b.WriteString(sectionStyle.Render("\n--- Last Activity ---") + "\n")

		b.WriteString(fmt.Sprintf("%s   %s (%s ago)\n", labelStyle.Render("Time:"),
			valueStyle.Render(lastMessage.Timestamp.Format(time.RFC822)),
			valueStyle.Render(time.Since(lastMessage.Timestamp).Round(time.Second).String())))

		roleColor := lipgloss.Color("86") // Cyan
		roleIcon := "🤖"
		if lastMessage.Role == "user" {
			roleColor = lipgloss.Color("220") // Yellow
			roleIcon = "👤"
		}
		b.WriteString(fmt.Sprintf("%s   %s\n", labelStyle.Render("Role:"), lipgloss.NewStyle().Foreground(roleColor).Render(fmt.Sprintf("%s %s", roleIcon, lastMessage.Role))))

		content := strings.TrimSpace(lastMessage.Content)
		// Basic wrapping or truncation
		contentWidth := width - 10
		if contentWidth < 20 {
			contentWidth = 40 // fallback
		}

		if len(content) > 300 {
			content = content[:300] + "..."
		}
		content = strings.ReplaceAll(content, "\n", " ↵ ")

		b.WriteString(fmt.Sprintf("%s %s\n", labelStyle.Render("Content:"), valueStyle.Render(content)))
	} else {
		b.WriteString(sectionStyle.Render("\n--- Activity Log ---") + "\n")
		b.WriteString(valueStyle.Render("Waiting for agent activity...") + "\n")
	}
	return b.String()
}

func refreshStatusCmd(sessionName string) tea.Cmd {
	return func() tea.Msg {
		if GetSessionStatus == nil {
			return fmt.Errorf("GetSessionStatus function is not set")
		}
		session, state, gitDiff, err := GetSessionStatus(sessionName)
		if err != nil {
			return err
		}
		return statusRefreshedMsg{session: session, agentState: state, gitDiffStat: gitDiff}
	}
}

// StartStatusDashboard starts the TUI dashboard for a specific session
var StartStatusDashboard = func(sessionName string) error {
	p := tea.NewProgram(NewStatusDashboardModel(sessionName), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
