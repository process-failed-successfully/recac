package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Styles
var (
	focusTitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FFF")).
			Background(lipgloss.Color("#6124DF")).
			Padding(0, 1).
			Bold(true)

	focusTimerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#04B575")).
			Bold(true).
			Height(3).
			Align(lipgloss.Center)

	focusTaskStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#241")).
			Italic(true)

	focusHelpStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#666")).
			MarginTop(1)
)

type TickMsg time.Time

type FocusState int

const (
	StateFocus FocusState = iota
	StateBreak
	StateFinished
)

type FocusModel struct {
	Duration      time.Duration
	Remaining     time.Duration
	TotalDuration time.Duration
	TaskName      string
	State         FocusState
	Paused        bool
	Quitting      bool

	// UI Components
	progress progress.Model
	width    int
	height   int
}

func NewFocusModel(duration time.Duration, taskName string) FocusModel {
	p := progress.New(progress.WithDefaultGradient())
	return FocusModel{
		Duration:      duration,
		Remaining:     duration,
		TotalDuration: duration,
		TaskName:      taskName,
		State:         StateFocus,
		progress:      p,
	}
}

func (m FocusModel) Init() tea.Cmd {
	return focusTickCmd()
}

func (m FocusModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.Quitting = true
			return m, tea.Quit
		case " ":
			m.Paused = !m.Paused
			return m, nil
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.progress.Width = msg.Width - 10
		if m.progress.Width > 80 {
			m.progress.Width = 80
		}
		return m, nil

	case TickMsg:
		if m.Paused || m.Remaining <= 0 {
			return m, focusTickCmd()
		}

		m.Remaining -= time.Second
		if m.Remaining <= 0 {
			m.Remaining = 0
			m.State = StateFinished
			// We could auto-quit or wait for user
			return m, tea.Quit
		}

		// Update progress
		percent := 1.0 - (float64(m.Remaining) / float64(m.TotalDuration))
		if percent < 0 {
			percent = 0
		}
		if percent > 1 {
			percent = 1
		}

		// The progress model update returns a command (sometimes for animation)
		// but here we just need to set the percentage next view.
		// Actually, bubbles/progress is stateless regarding percentage in Update,
		// you just set it in View usually, OR you send a SetPercent msg.
		// Let's just calculate it in View or pass a cmd if needed.
		// Standard usage is m.progress.ViewAs(percent).
		return m, focusTickCmd()
	}

	return m, nil
}

func (m FocusModel) View() string {
	if m.Quitting {
		return ""
	}

	var s strings.Builder

	// Title
	title := "🍅 RECAC FOCUS"
	if m.Paused {
		title += " (PAUSED)"
	}
	s.WriteString(focusTitleStyle.Render(title) + "\n\n")

	// Task
	if m.TaskName != "" {
		s.WriteString(fmt.Sprintf("Task: %s\n", focusTaskStyle.Render(m.TaskName)))
	}
	s.WriteString("\n")

	// Timer
	minutes := int(m.Remaining.Minutes())
	seconds := int(m.Remaining.Seconds()) % 60
	timerStr := fmt.Sprintf("%02d:%02d", minutes, seconds)

	// Big ASCII art or just big text?
	// For now, big text via style.
	// Actually, let's just make it look clean.
	s.WriteString(focusTimerStyle.Render(timerStr) + "\n\n")

	// Progress Bar
	percent := 1.0 - (float64(m.Remaining) / float64(m.TotalDuration))
	s.WriteString(m.progress.ViewAs(percent) + "\n")

	// Help
	s.WriteString(focusHelpStyle.Render("(q) quit • (space) pause/resume"))

	// Center vertically if we have height
	if m.height > 0 {
		return lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center, s.String())
	}

	return s.String()
}

func focusTickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return TickMsg(t)
	})
}
