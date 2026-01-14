package ui

import (
	"fmt"
	"recac/internal/model"
	"strings"
	"text/tabwriter"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/pflag"
)

// SessionFetcher is a function that can fetch unified sessions.
// This is used to inject the fetching logic from the cmd package.
var SessionFetcher func(flags *pflag.FlagSet) ([]model.UnifiedSession, []string, error)

type psDashboardModel struct {
	sessions   []model.UnifiedSession
	warnings   []string
	lastUpdate time.Time
	flags      *pflag.FlagSet
	err        error
}

type psTickMsg time.Time
type psSessionsRefreshedMsg struct {
	sessions []model.UnifiedSession
	warnings []string
}

func NewPsDashboardModel(flags *pflag.FlagSet) psDashboardModel {
	return psDashboardModel{
		flags: flags,
	}
}

func (m psDashboardModel) Init() tea.Cmd {
	return tea.Batch(m.refreshSessionsCmd(), tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
		return psTickMsg(t)
	}))
}

func (m psDashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "r":
			return m, m.refreshSessionsCmd()
		}
	case psTickMsg:
		return m, m.refreshSessionsCmd()
	case psSessionsRefreshedMsg:
		m.sessions = msg.sessions
		m.warnings = msg.warnings
		m.lastUpdate = time.Now()
		return m, nil
	case error:
		m.err = msg
		return m, nil
	}
	return m, nil
}

func (m psDashboardModel) View() string {
	if m.err != nil {
		return fmt.Sprintf("Error: %v", m.err)
	}

	var s strings.Builder
	s.WriteString(summaryTitleStyle.Render("📊 RECAC `ps` Dashboard") + "\n")
	s.WriteString(fmt.Sprintf("Last updated: %s (press 'r' to refresh, 'q' to quit)\n", m.lastUpdate.Format(time.RFC1123)))

	if len(m.sessions) == 0 {
		s.WriteString("\nNo sessions found.")
		return s.String()
	}

	s.WriteString(m.renderSessionTable())

	if len(m.warnings) > 0 {
		warningStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("3")).Bold(true)
		s.WriteString("\n" + warningStyle.Render("Warnings:") + "\n")
		for _, w := range m.warnings {
			s.WriteString("- " + w + "\n")
		}
	}

	return s.String()
}

func (m *psDashboardModel) renderSessionTable() string {
	var b strings.Builder
	w := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)

	showCosts, _ := m.flags.GetBool("costs")
	header := "NAME\tSTATUS\tLOCATION\tSTARTED\tDURATION"
	if showCosts {
		header += "\tPROMPT_TOKENS\tCOMPLETION_TOKENS\tTOTAL_TOKENS\tCOST"
	}
	fmt.Fprintln(w, headerStyle.Render(header))

	for _, s := range m.sessions {
		started := s.StartTime.Format("2006-01-02 15:04:05")
		var duration string
		if s.EndTime.IsZero() {
			duration = time.Since(s.StartTime).Round(time.Second).String()
		} else {
			duration = s.EndTime.Sub(s.StartTime).Round(time.Second).String()
		}

		baseOutput := fmt.Sprintf("%s\t%s\t%s\t%s\t%s",
			s.Name, s.Status, s.Location, started, duration)

		if showCosts {
			if s.HasCost {
				fmt.Fprintf(w, "%s\t%d\t%d\t%d\t$%.6f\n",
					baseOutput, s.Tokens.TotalPromptTokens, s.Tokens.TotalResponseTokens, s.Tokens.TotalTokens, s.Cost)
			} else {
				fmt.Fprintf(w, "%s\tN/A\tN/A\tN/A\tN/A\n", baseOutput)
			}
		} else {
			fmt.Fprintf(w, "%s\n", baseOutput)
		}
	}

	w.Flush()
	return b.String()
}

func (m *psDashboardModel) refreshSessionsCmd() tea.Cmd {
	return func() tea.Msg {
		if SessionFetcher == nil {
			return fmt.Errorf("SessionFetcher is not initialized")
		}
		sessions, warnings, err := SessionFetcher(m.flags)
		if err != nil {
			return err
		}
		return psSessionsRefreshedMsg{sessions: sessions, warnings: warnings}
	}
}

func StartPsDashboard(flags *pflag.FlagSet, fetcher func(*pflag.FlagSet) ([]model.UnifiedSession, []string, error)) error {
	SessionFetcher = fetcher
	p := tea.NewProgram(NewPsDashboardModel(flags))
	if _, err := p.Run(); err != nil {
		return err
	}
	return nil
}
