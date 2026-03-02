package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type GitStatus struct {
	Branch         string
	Dirty          bool
	Ahead          int
	Behind         int
	LastCommitMsg  string
	LastCommitHash string
	Unpushed       bool
}

type TodoSummary struct {
	Count    int
	Critical int
}

type RecentSession struct {
	Name    string
	Status  string
	Time    time.Time
	Elapsed time.Duration
}

type SystemInfo struct {
	OS          string
	MemoryUsage string
	CPUUsage    string
	Uptime      time.Duration
}

type HomeModel struct {
	Git      GitStatus
	Todos    TodoSummary
	Sessions []RecentSession
	System   SystemInfo
	Width    int
	Height   int
}

func NewHomeModel(git GitStatus, todos TodoSummary, sessions []RecentSession, sys SystemInfo) HomeModel {
	return HomeModel{
		Git:      git,
		Todos:    todos,
		Sessions: sessions,
		System:   sys,
	}
}

func (m HomeModel) Init() tea.Cmd {
	return nil
}

func (m HomeModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
	}
	return m, nil
}

func (m HomeModel) View() string {
	// Styles
	boxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1, 2).
		MarginRight(1)

	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Bold(true).
		MarginBottom(1)

	// Git Section
	gitContent := fmt.Sprintf("Branch: %s\n", m.Git.Branch)
	if m.Git.Dirty {
		gitContent += "Dirty: Yes (Uncommitted changes)\n"
	} else {
		gitContent += "Dirty: No\n"
	}
	if len(m.Git.LastCommitMsg) > 30 {
		m.Git.LastCommitMsg = m.Git.LastCommitMsg[:27] + "..."
	}
	gitContent += fmt.Sprintf("Last Commit: %s (%s)\n", m.Git.LastCommitMsg, m.Git.LastCommitHash)

	gitBox := boxStyle.Copy().Render(
		headerStyle.Render("Git Status") + "\n" + gitContent,
	)

	// Sessions Section
	sessionsContent := ""
	if len(m.Sessions) == 0 {
		sessionsContent = "No recent sessions."
	} else {
		for _, s := range m.Sessions {
			statusIcon := "🟢"
			if s.Status == "failed" || s.Status == "error" {
				statusIcon = "🔴"
			} else if s.Status == "running" {
				statusIcon = "🏃"
			}
			sessionsContent += fmt.Sprintf("%s %s (%s)\n", statusIcon, s.Name, s.Time.Format("15:04"))
		}
	}

	sessionsBox := boxStyle.Copy().Render(
		headerStyle.Render("Recent Agent Sessions") + "\n" + sessionsContent,
	)

	// Todos Section
	todoContent := fmt.Sprintf("Total TODOs: %d\n", m.Todos.Count)
	if m.Todos.Critical > 0 {
		todoContent += fmt.Sprintf("Critical: %d\n", m.Todos.Critical)
	}

	todoBox := boxStyle.Copy().Render(
		headerStyle.Render("Tasks & TODOs") + "\n" + todoContent,
	)

	// Layout
	// Use simple vertical stacking or horizontal if width permits
	// For now, vertical stack
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("86")).Render("RECAC Developer Home"),
		"",
		lipgloss.JoinHorizontal(lipgloss.Top, gitBox, sessionsBox),
		todoBox,
		"\nPress 'q' or 'ctrl+c' to quit.",
	)
}
