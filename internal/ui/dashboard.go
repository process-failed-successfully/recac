package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Log Levels/Types
type LogType int

const (
	LogInfo LogType = iota
	LogThought
	LogError
	LogSuccess
)

// Styles
var (
	paneStyle = lipgloss.NewStyle().
		Border(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("63")) // Purple-ish

	footerStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF")).
		Background(lipgloss.Color("#7D56F4")).
		Padding(0, 1)

	// Log Styles
	logInfoStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("252")) // Light Gray

	logThoughtStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("86")) // Cyan/Teal for thoughts

	logErrorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("196")). // Red
			Bold(true)

	logSuccessStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("46")). // Green
			Bold(true)
)

type LogEntry struct {
	Type    LogType
	Message string
}

type LogMsg LogEntry
type ProgressMsg float64

type DashboardModel struct {
	width    int
	height   int
	ready    bool
	logs     []LogEntry
	progress progress.Model
	viewport viewport.Model
}

func NewDashboardModel() DashboardModel {
	prog := progress.New(progress.WithDefaultGradient())
	return DashboardModel{
		logs:     make([]LogEntry, 0),
		progress: prog,
	}
}

func (m DashboardModel) Init() tea.Cmd {
	return nil
}

func (m DashboardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			return m, tea.Quit
		}
		// Pass keys to viewport for scrolling
		m.viewport, cmd = m.viewport.Update(msg)
		cmds = append(cmds, cmd)

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		
		headerHeight := 1
		footerHeight := 2
		mainHeight := m.height - footerHeight - headerHeight
		halfWidth := m.width/2 - 2

		if !m.ready {
			m.viewport = viewport.New(halfWidth, mainHeight-2)
			m.ready = true
		} else {
			m.viewport.Width = halfWidth
			m.viewport.Height = mainHeight - 2
		}

		m.progress.Width = msg.Width - 20 

	case LogMsg:
		m.logs = append(m.logs, LogEntry(msg))
		if len(m.logs) > 200 { // Increased buffer
			m.logs = m.logs[len(m.logs)-200:]
		}
		m.updateViewport()

	case ProgressMsg:
		cmd = m.progress.SetPercent(float64(msg))
		cmds = append(cmds, cmd)
	case progress.FrameMsg:
		newModel, cmd := m.progress.Update(msg)
		m.progress = newModel.(progress.Model)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *DashboardModel) updateViewport() {
	var logBuilder strings.Builder
	halfWidth := m.width/2 - 2
	
	for _, log := range m.logs {
		var style lipgloss.Style
		prefix := ""
		switch log.Type {
		case LogThought:
			style = logThoughtStyle
			prefix = "• "
		case LogError:
			style = logErrorStyle
			prefix = "✗ "
		case LogSuccess:
			style = logSuccessStyle
			prefix = "✓ "
		default:
			style = logInfoStyle
			prefix = "  "
		}
		style = style.Width(halfWidth - 2) 
		logBuilder.WriteString(style.Render(prefix + log.Message))
		logBuilder.WriteString("\n")
	}
	m.viewport.SetContent(logBuilder.String())
	m.viewport.GotoBottom()
}

func (m DashboardModel) View() string {
	if !m.ready {
		return "Initializing..."
	}

	// Header
	headerStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#FFF")).
		Background(lipgloss.Color("#7D56F4")). // Brand Color (matches footer)
		Bold(true).
		Padding(0, 1).
		Width(m.width)

	header := headerStyle.Render("RECAC: Autonomous Coding Agent - Recac v0.1.0")

	// Adjust main height for header and footer
	headerHeight := 1
	footerHeight := 2
	mainHeight := m.height - footerHeight - headerHeight
	if mainHeight < 0 {
		mainHeight = 0
	}

	halfWidth := m.width/2 - 2
	if halfWidth < 0 {
		halfWidth = 0
	}

	leftPane := paneStyle.
		Width(halfWidth).
		Height(mainHeight - 2).
		Render("Left Pane\n(Project/Tasks)")

	rightPane := paneStyle.
		Width(halfWidth).
		Height(mainHeight - 2).
		Render(m.viewport.View())

	// Footer View
	progView := m.progress.View()
	footerText := "Status: Ready"
	
	footer := footerStyle.
		Width(m.width).
		Render(lipgloss.JoinVertical(lipgloss.Left, footerText, progView))

	mainView := lipgloss.JoinHorizontal(lipgloss.Top, leftPane, rightPane)

	return lipgloss.JoinVertical(lipgloss.Left, header, mainView, footer)
}
